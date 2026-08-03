package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type generatedPublicationKind uint8

const (
	generatedPublicationWrite generatedPublicationKind = iota
	generatedPublicationRemove
)

type generatedPublicationPhase string

const (
	generatedPublicationStage    generatedPublicationPhase = "stage"
	generatedPublicationCommit   generatedPublicationPhase = "commit"
	generatedPublicationRollback generatedPublicationPhase = "rollback"
)

type generatedPublicationAction string

const (
	generatedPublicationCreateDirectory generatedPublicationAction = "create-directory"
	generatedPublicationCreateStage     generatedPublicationAction = "create-stage"
	generatedPublicationWriteStage      generatedPublicationAction = "write-stage"
	generatedPublicationChmodStage      generatedPublicationAction = "chmod-stage"
	generatedPublicationCloseStage      generatedPublicationAction = "close-stage"
	generatedPublicationReplaceOutput   generatedPublicationAction = "replace-output"
	generatedPublicationRemoveOutput    generatedPublicationAction = "remove-output"
	generatedPublicationRestoreOutput   generatedPublicationAction = "restore-output"
	generatedPublicationDiscardOutput   generatedPublicationAction = "discard-output"
)

type generatedPublicationStep struct {
	phase  generatedPublicationPhase
	action generatedPublicationAction
	path   string
}

type generatedPublicationHook func(generatedPublicationStep) error

type generatedPublicationState struct {
	exists  bool
	info    os.FileInfo
	content []byte
	mode    os.FileMode
	digest  [sha256.Size]byte
}

type generatedPublicationEntry struct {
	kind      generatedPublicationKind
	path      string
	content   []byte
	prior     generatedPublicationState
	stagePath string
	mutated   bool
}

type generatedPublicationPlan struct {
	root                string
	entries             []generatedPublicationEntry
	requiredDirectories []string
	createdDirectories  []string
	hook                generatedPublicationHook
}

func publishGeneratedSourceSet(
	root string,
	generated []generatedGOXFile,
	removals []string,
) error {
	return publishGeneratedSourceSetWithHook(root, generated, removals, nil)
}

func publishGeneratedSourceSetWithHook(
	root string,
	generated []generatedGOXFile,
	removals []string,
	hook generatedPublicationHook,
) error {
	plan, err := planGeneratedSourcePublication(root, generated, removals, hook)
	if err != nil {
		return err
	}
	if err := plan.stage(); err != nil {
		return plan.failBeforeCommit(err)
	}
	if err := plan.revalidate(); err != nil {
		return plan.failBeforeCommit(err)
	}
	if err := plan.commit(); err != nil {
		return err
	}
	return nil
}

func planGeneratedSourcePublication(
	root string,
	generated []generatedGOXFile,
	removals []string,
	hook generatedPublicationHook,
) (*generatedPublicationPlan, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve generated publication root %s: %w", root, err)
	}
	root = filepath.Clean(root)
	plan := &generatedPublicationPlan{
		root: root,
		hook: hook,
		entries: make(
			[]generatedPublicationEntry,
			0,
			len(generated)+len(removals),
		),
	}
	for _, file := range generated {
		path, err := filepath.Abs(file.path)
		if err != nil {
			return nil, fmt.Errorf("resolve generated output %s: %w", file.path, err)
		}
		plan.entries = append(plan.entries, generatedPublicationEntry{
			kind:    generatedPublicationWrite,
			path:    filepath.Clean(path),
			content: append([]byte(nil), file.content...),
		})
	}
	for _, removal := range removals {
		path, err := filepath.Abs(removal)
		if err != nil {
			return nil, fmt.Errorf("resolve inactive generated output %s: %w", removal, err)
		}
		plan.entries = append(plan.entries, generatedPublicationEntry{
			kind: generatedPublicationRemove,
			path: filepath.Clean(path),
		})
	}
	sort.Slice(plan.entries, func(left, right int) bool {
		return plan.entries[left].path < plan.entries[right].path
	})

	identities := make(map[string]string, len(plan.entries))
	filtered := plan.entries[:0]
	directories := make(map[string]struct{})
	for index := range plan.entries {
		entry := &plan.entries[index]
		if err := validateGeneratedPublicationPath(root, entry.path); err != nil {
			return nil, err
		}
		identity, err := generatedPublicationPathIdentity(entry.path)
		if err != nil {
			return nil, fmt.Errorf("resolve generated publication identity for %s: %w", entry.path, err)
		}
		if previous, duplicate := identities[identity]; duplicate {
			return nil, fmt.Errorf(
				"generated publication paths %s and %s refer to the same destination",
				previous,
				entry.path,
			)
		}
		identities[identity] = entry.path

		state, err := inspectGeneratedPublicationState(entry.path)
		if err != nil {
			if entry.kind == generatedPublicationWrite {
				return nil, fmt.Errorf("write %s: %w", entry.path, err)
			}
			return nil, fmt.Errorf("remove inactive generated output %s: %w", entry.path, err)
		}
		if entry.kind == generatedPublicationRemove {
			if !state.exists {
				continue
			}
			if !bytes.HasPrefix(state.content, []byte(generatedGOXFileHeader)) {
				return nil, fmt.Errorf(
					"refuse to remove inactive generated output %s: file is not managed by goxc",
					entry.path,
				)
			}
		} else {
			directories[filepath.Dir(entry.path)] = struct{}{}
		}
		entry.prior = state
		filtered = append(filtered, *entry)
	}
	plan.entries = filtered
	plan.requiredDirectories = make([]string, 0, len(directories))
	for directory := range directories {
		plan.requiredDirectories = append(plan.requiredDirectories, directory)
	}
	sort.Slice(plan.requiredDirectories, func(left, right int) bool {
		leftDepth := generatedPublicationPathDepth(plan.requiredDirectories[left])
		rightDepth := generatedPublicationPathDepth(plan.requiredDirectories[right])
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return plan.requiredDirectories[left] < plan.requiredDirectories[right]
	})
	return plan, nil
}

func validateGeneratedPublicationPath(root, path string) error {
	if err := validatePathBelowRoot(
		root,
		path,
		"generated publication output",
		true,
	); err != nil {
		return err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve generated output %s below %s: %w", path, root, err)
	}
	if relative == "." {
		return fmt.Errorf("generated publication output %s must be below %s", path, root)
	}
	return nil
}

func generatedPublicationPathIdentity(path string) (string, error) {
	identity, err := canonicalPathForComparison(path)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		identity = strings.ToLower(identity)
	}
	return identity, nil
}

func generatedPublicationPathDepth(path string) int {
	volume := filepath.VolumeName(path)
	path = strings.TrimPrefix(path, volume)
	return len(strings.FieldsFunc(path, func(value rune) bool {
		return value == '/' || value == '\\'
	}))
}

func inspectGeneratedPublicationState(path string) (generatedPublicationState, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return generatedPublicationState{}, nil
	}
	if err != nil {
		return generatedPublicationState{}, fmt.Errorf("inspect destination %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return generatedPublicationState{}, fmt.Errorf(
			"destination %s is a symlink; symlink paths are not supported",
			path,
		)
	}
	if info.IsDir() {
		return generatedPublicationState{}, fmt.Errorf("destination %s is a directory", path)
	}
	if !info.Mode().IsRegular() {
		return generatedPublicationState{}, fmt.Errorf("destination %s is not a regular file", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return generatedPublicationState{}, fmt.Errorf("read destination %s: %w", path, err)
	}
	verified, err := os.Lstat(path)
	if err != nil {
		return generatedPublicationState{}, fmt.Errorf("reinspect destination %s: %w", path, err)
	}
	if verified.Mode()&os.ModeSymlink != 0 || !verified.Mode().IsRegular() {
		return generatedPublicationState{}, fmt.Errorf(
			"destination %s changed while recording publication state",
			path,
		)
	}
	if !os.SameFile(info, verified) {
		return generatedPublicationState{}, fmt.Errorf(
			"destination %s changed identity while recording publication state",
			path,
		)
	}
	return generatedPublicationState{
		exists:  true,
		info:    verified,
		content: content,
		mode:    verified.Mode().Perm(),
		digest:  sha256.Sum256(content),
	}, nil
}

func (plan *generatedPublicationPlan) stage() error {
	for _, directory := range plan.requiredDirectories {
		if err := plan.ensureDirectory(directory); err != nil {
			return err
		}
	}
	for index := range plan.entries {
		entry := &plan.entries[index]
		if entry.kind != generatedPublicationWrite {
			continue
		}
		if err := plan.stageEntry(entry); err != nil {
			return err
		}
	}
	return nil
}

func (plan *generatedPublicationPlan) ensureDirectory(directory string) error {
	if err := validatePathBelowRoot(
		plan.root,
		directory,
		"generated publication directory",
		true,
	); err != nil {
		return err
	}
	missing := make([]string, 0)
	current := directory
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf(
					"generated publication directory %s is a symlink; symlink paths are not supported",
					current,
				)
			}
			if !info.IsDir() {
				return fmt.Errorf("generated publication directory %s is not a directory", current)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect generated publication directory %s: %w", current, err)
		}
		missing = append(missing, current)
		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("no existing ancestor found for generated publication directory %s", directory)
		}
		current = parent
	}
	for index := len(missing) - 1; index >= 0; index-- {
		path := missing[index]
		if err := plan.before(generatedPublicationStep{
			phase:  generatedPublicationStage,
			action: generatedPublicationCreateDirectory,
			path:   path,
		}); err != nil {
			return err
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			return fmt.Errorf("create generated publication directory %s: %w", path, err)
		}
		plan.createdDirectories = append(plan.createdDirectories, path)
	}
	return nil
}

func (plan *generatedPublicationPlan) stageEntry(entry *generatedPublicationEntry) (resultErr error) {
	if err := plan.before(generatedPublicationStep{
		phase:  generatedPublicationStage,
		action: generatedPublicationCreateStage,
		path:   entry.path,
	}); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(entry.path), ".goframe-publish-*")
	if err != nil {
		return fmt.Errorf("create staged generated output for %s: %w", entry.path, err)
	}
	entry.stagePath = file.Name()
	closed := false
	defer func() {
		if closed {
			return
		}
		if err := file.Close(); err != nil {
			closeErr := fmt.Errorf("close staged generated output for %s: %w", entry.path, err)
			if resultErr == nil {
				resultErr = closeErr
			} else {
				resultErr = errors.Join(resultErr, closeErr)
			}
		}
	}()

	if err := plan.before(generatedPublicationStep{
		phase:  generatedPublicationStage,
		action: generatedPublicationWriteStage,
		path:   entry.path,
	}); err != nil {
		return err
	}
	written, err := file.Write(entry.content)
	if err != nil {
		return fmt.Errorf("write staged generated output for %s: %w", entry.path, err)
	}
	if written != len(entry.content) {
		return fmt.Errorf("write staged generated output for %s: %w", entry.path, io.ErrShortWrite)
	}
	if err := plan.before(generatedPublicationStep{
		phase:  generatedPublicationStage,
		action: generatedPublicationChmodStage,
		path:   entry.path,
	}); err != nil {
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		return fmt.Errorf("chmod staged generated output for %s: %w", entry.path, err)
	}
	if err := plan.before(generatedPublicationStep{
		phase:  generatedPublicationStage,
		action: generatedPublicationCloseStage,
		path:   entry.path,
	}); err != nil {
		closeErr := file.Close()
		closed = true
		if closeErr != nil {
			return errors.Join(err, fmt.Errorf(
				"close staged generated output for %s: %w",
				entry.path,
				closeErr,
			))
		}
		return err
	}
	if err := file.Close(); err != nil {
		closed = true
		return fmt.Errorf("close staged generated output for %s: %w", entry.path, err)
	}
	closed = true
	return nil
}

func (plan *generatedPublicationPlan) revalidate() error {
	for index := range plan.entries {
		if err := plan.revalidateEntry(&plan.entries[index]); err != nil {
			return err
		}
	}
	return nil
}

func (plan *generatedPublicationPlan) revalidateEntry(entry *generatedPublicationEntry) error {
	if err := validateGeneratedPublicationPath(plan.root, entry.path); err != nil {
		return err
	}
	current, err := inspectGeneratedPublicationState(entry.path)
	if err != nil {
		return fmt.Errorf("revalidate generated publication output %s: %w", entry.path, err)
	}
	if entry.prior.exists != current.exists {
		return fmt.Errorf("generated publication output %s changed existence after planning", entry.path)
	}
	if !entry.prior.exists {
		return nil
	}
	if !os.SameFile(entry.prior.info, current.info) {
		return fmt.Errorf("generated publication output %s changed identity after planning", entry.path)
	}
	if entry.prior.digest != current.digest {
		return fmt.Errorf("generated publication output %s changed content after planning", entry.path)
	}
	if entry.prior.mode.Perm() != current.mode.Perm() {
		return fmt.Errorf("generated publication output %s changed permissions after planning", entry.path)
	}
	if entry.kind == generatedPublicationRemove &&
		!bytes.HasPrefix(current.content, []byte(generatedGOXFileHeader)) {
		return fmt.Errorf(
			"refuse to remove inactive generated output %s: file is not managed by goxc",
			entry.path,
		)
	}
	return nil
}

func (plan *generatedPublicationPlan) commit() error {
	for index := range plan.entries {
		entry := &plan.entries[index]
		if err := plan.revalidateEntry(entry); err != nil {
			return plan.rollback(err)
		}
		var err error
		switch entry.kind {
		case generatedPublicationWrite:
			err = plan.before(generatedPublicationStep{
				phase:  generatedPublicationCommit,
				action: generatedPublicationReplaceOutput,
				path:   entry.path,
			})
			if err == nil {
				err = os.Rename(entry.stagePath, entry.path)
			}
			if err != nil {
				err = fmt.Errorf("publish generated output %s: %w", entry.path, err)
			} else {
				entry.stagePath = ""
				entry.mutated = true
			}
		case generatedPublicationRemove:
			err = plan.before(generatedPublicationStep{
				phase:  generatedPublicationCommit,
				action: generatedPublicationRemoveOutput,
				path:   entry.path,
			})
			if err == nil {
				err = os.Remove(entry.path)
			}
			removed := err == nil
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
			if err != nil {
				err = fmt.Errorf("remove inactive generated output %s: %w", entry.path, err)
			} else if removed {
				entry.mutated = true
			}
		default:
			err = fmt.Errorf("unsupported generated publication operation for %s", entry.path)
		}
		if err != nil {
			return plan.rollback(err)
		}
	}
	return nil
}

func (plan *generatedPublicationPlan) rollback(primary error) error {
	var rollbackErrors []error
	for index := len(plan.entries) - 1; index >= 0; index-- {
		entry := &plan.entries[index]
		if !entry.mutated {
			continue
		}
		if err := plan.restoreEntry(entry); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	rollbackErrors = append(rollbackErrors, plan.cleanupStagedFiles()...)
	rollbackErrors = append(rollbackErrors, plan.cleanupCreatedDirectories()...)
	if len(rollbackErrors) == 0 {
		return primary
	}
	return errors.Join(
		primary,
		fmt.Errorf(
			"rollback generated source publication: %w",
			errors.Join(rollbackErrors...),
		),
	)
}

func (plan *generatedPublicationPlan) restoreEntry(entry *generatedPublicationEntry) error {
	if entry.prior.exists {
		if err := plan.before(generatedPublicationStep{
			phase:  generatedPublicationRollback,
			action: generatedPublicationRestoreOutput,
			path:   entry.path,
		}); err != nil {
			return fmt.Errorf("restore generated publication output %s: %w", entry.path, err)
		}
		if err := writeGeneratedPublicationSnapshot(
			plan.root,
			entry.path,
			entry.prior.content,
			entry.prior.mode,
		); err != nil {
			return fmt.Errorf("restore generated publication output %s: %w", entry.path, err)
		}
		entry.mutated = false
		return nil
	}
	if err := plan.before(generatedPublicationStep{
		phase:  generatedPublicationRollback,
		action: generatedPublicationDiscardOutput,
		path:   entry.path,
	}); err != nil {
		return fmt.Errorf("discard generated publication output %s: %w", entry.path, err)
	}
	if err := validateGeneratedPublicationPath(plan.root, entry.path); err != nil {
		return fmt.Errorf("discard generated publication output %s: %w", entry.path, err)
	}
	if err := os.Remove(entry.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("discard generated publication output %s: %w", entry.path, err)
	}
	entry.mutated = false
	return nil
}

func writeGeneratedPublicationSnapshot(
	root,
	path string,
	content []byte,
	mode os.FileMode,
) error {
	if err := validateGeneratedPublicationPath(root, path); err != nil {
		return err
	}
	if current, err := os.Lstat(path); err == nil {
		if current.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination %s is a symlink; symlink paths are not supported", path)
		}
		if !current.Mode().IsRegular() {
			return fmt.Errorf("destination %s is not a regular file", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect destination %s: %w", path, err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".goframe-rollback-*")
	if err != nil {
		return fmt.Errorf("create rollback file for %s: %w", path, err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	written, writeErr := file.Write(content)
	if writeErr == nil && written != len(content) {
		writeErr = io.ErrShortWrite
	}
	chmodErr := file.Chmod(mode.Perm())
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write rollback file for %s: %w", path, writeErr)
	}
	if chmodErr != nil {
		return fmt.Errorf("chmod rollback file for %s: %w", path, chmodErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close rollback file for %s: %w", path, closeErr)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s during rollback: %w", path, err)
	}
	return nil
}

func (plan *generatedPublicationPlan) failBeforeCommit(primary error) error {
	cleanupErrors := plan.cleanupStagedFiles()
	cleanupErrors = append(cleanupErrors, plan.cleanupCreatedDirectories()...)
	if len(cleanupErrors) == 0 {
		return primary
	}
	return errors.Join(
		primary,
		fmt.Errorf(
			"cleanup generated source publication: %w",
			errors.Join(cleanupErrors...),
		),
	)
}

func (plan *generatedPublicationPlan) cleanupStagedFiles() []error {
	var cleanupErrors []error
	for index := range plan.entries {
		stagePath := plan.entries[index].stagePath
		if stagePath == "" {
			continue
		}
		if err := os.Remove(stagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf(
				"remove staged generated output %s: %w",
				stagePath,
				err,
			))
		}
		plan.entries[index].stagePath = ""
	}
	return cleanupErrors
}

func (plan *generatedPublicationPlan) cleanupCreatedDirectories() []error {
	var cleanupErrors []error
	for index := len(plan.createdDirectories) - 1; index >= 0; index-- {
		directory := plan.createdDirectories[index]
		entries, err := os.ReadDir(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf(
				"inspect transaction-created directory %s: %w",
				directory,
				err,
			))
			continue
		}
		if len(entries) != 0 {
			continue
		}
		if err := os.Remove(directory); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf(
				"remove transaction-created directory %s: %w",
				directory,
				err,
			))
		}
	}
	plan.createdDirectories = nil
	return cleanupErrors
}

func (plan *generatedPublicationPlan) before(step generatedPublicationStep) error {
	if plan.hook == nil {
		return nil
	}
	if err := plan.hook(step); err != nil {
		return fmt.Errorf(
			"generated publication %s %s for %s: %w",
			step.phase,
			step.action,
			step.path,
			err,
		)
	}
	return nil
}
