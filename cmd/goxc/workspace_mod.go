package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type workspaceModuleDirectives struct {
	Requires []workspaceRequireDirective
	Replaces []workspaceReplaceDirective
}

type workspaceRequireDirective struct {
	Path     string
	Version  string
	Indirect bool
}

type workspaceReplaceDirective struct {
	OldPath    string
	OldVersion string
	NewPath    string
	NewVersion string
}

func readWorkspaceModuleDirectives(moduleRoot string) (workspaceModuleDirectives, error) {
	if moduleRoot == "" {
		return workspaceModuleDirectives{}, nil
	}
	path := filepath.Join(moduleRoot, "go.mod")
	file, err := os.Open(path)
	if err != nil {
		return workspaceModuleDirectives{}, fmt.Errorf("read app module directives from %s: %w", path, err)
	}
	defer file.Close()

	var directives workspaceModuleDirectives
	block := ""
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		fields, err := goModFields(rawLine)
		if err != nil {
			return workspaceModuleDirectives{}, fmt.Errorf("parse %s:%d: %w", path, lineNumber, err)
		}
		if len(fields) == 0 {
			continue
		}
		if block != "" {
			if fields[0] == ")" {
				block = ""
				continue
			}
			switch block {
			case "require":
				require, ok, err := parseWorkspaceRequire(fields, rawLine)
				if err != nil {
					return workspaceModuleDirectives{}, fmt.Errorf("parse %s:%d: %w", path, lineNumber, err)
				}
				if ok {
					directives.Requires = append(directives.Requires, require)
				}
			case "replace":
				replace, ok, err := parseWorkspaceReplace(fields)
				if err != nil {
					return workspaceModuleDirectives{}, fmt.Errorf("parse %s:%d: %w", path, lineNumber, err)
				}
				if ok {
					replace, err = rewriteWorkspaceReplace(moduleRoot, replace)
					if err != nil {
						return workspaceModuleDirectives{}, fmt.Errorf("parse %s:%d: %w", path, lineNumber, err)
					}
					directives.Replaces = append(directives.Replaces, replace)
				}
			}
			continue
		}
		switch fields[0] {
		case "require":
			if len(fields) == 2 && fields[1] == "(" {
				block = "require"
				continue
			}
			require, ok, err := parseWorkspaceRequire(fields[1:], rawLine)
			if err != nil {
				return workspaceModuleDirectives{}, fmt.Errorf("parse %s:%d: %w", path, lineNumber, err)
			}
			if ok {
				directives.Requires = append(directives.Requires, require)
			}
		case "replace":
			if len(fields) == 2 && fields[1] == "(" {
				block = "replace"
				continue
			}
			replace, ok, err := parseWorkspaceReplace(fields[1:])
			if err != nil {
				return workspaceModuleDirectives{}, fmt.Errorf("parse %s:%d: %w", path, lineNumber, err)
			}
			if ok {
				replace, err = rewriteWorkspaceReplace(moduleRoot, replace)
				if err != nil {
					return workspaceModuleDirectives{}, fmt.Errorf("parse %s:%d: %w", path, lineNumber, err)
				}
				directives.Replaces = append(directives.Replaces, replace)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return workspaceModuleDirectives{}, fmt.Errorf("read app module directives from %s: %w", path, err)
	}
	if block != "" {
		return workspaceModuleDirectives{}, fmt.Errorf("parse %s: unterminated %s block", path, block)
	}
	return directives, nil
}

func parseWorkspaceRequire(fields []string, rawLine string) (workspaceRequireDirective, bool, error) {
	if len(fields) < 2 {
		return workspaceRequireDirective{}, false, fmt.Errorf("malformed require directive")
	}
	if fields[0] == canonicalModulePath {
		return workspaceRequireDirective{}, false, nil
	}
	return workspaceRequireDirective{
		Path:     fields[0],
		Version:  fields[1],
		Indirect: strings.Contains(rawLine, "// indirect"),
	}, true, nil
}

func parseWorkspaceReplace(fields []string) (workspaceReplaceDirective, bool, error) {
	arrow := -1
	for index, field := range fields {
		if field == "=>" {
			arrow = index
			break
		}
	}
	if arrow <= 0 || arrow+1 >= len(fields) {
		return workspaceReplaceDirective{}, false, fmt.Errorf("malformed replace directive")
	}
	if arrow > 2 || len(fields)-arrow-1 > 2 {
		return workspaceReplaceDirective{}, false, fmt.Errorf("malformed replace directive")
	}
	replace := workspaceReplaceDirective{
		OldPath: fields[0],
		NewPath: fields[arrow+1],
	}
	if arrow == 2 {
		replace.OldVersion = fields[1]
	}
	if len(fields)-arrow-1 == 2 {
		replace.NewVersion = fields[arrow+2]
	}
	if replace.OldPath == canonicalModulePath {
		return workspaceReplaceDirective{}, false, nil
	}
	return replace, true, nil
}

func rewriteWorkspaceReplace(moduleRoot string, replace workspaceReplaceDirective) (workspaceReplaceDirective, error) {
	if replace.NewVersion != "" || !isLocalReplacePath(replace.NewPath) {
		return replace, nil
	}
	target := filepath.FromSlash(replace.NewPath)
	if !filepath.IsAbs(target) {
		target = filepath.Join(moduleRoot, target)
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return workspaceReplaceDirective{}, fmt.Errorf("resolve replace target %s: %w", replace.NewPath, err)
	}
	replace.NewPath = filepath.ToSlash(absolute)
	return replace, nil
}

func isLocalReplacePath(path string) bool {
	path = filepath.ToSlash(path)
	return path == "." || path == ".." || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.HasPrefix(path, "/") || goModPathHasDriveRoot(path)
}

func goModPathHasDriveRoot(path string) bool {
	if len(path) < 3 || path[1] != ':' || path[2] != '/' {
		return false
	}
	drive := path[0]
	return (drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')
}

func writeWorkspaceRequires(content *strings.Builder, requires []workspaceRequireDirective) {
	if len(requires) == 0 {
		return
	}
	content.WriteString("require (\n")
	for _, require := range requires {
		content.WriteString("\t")
		content.WriteString(goModToken(require.Path))
		content.WriteString(" ")
		content.WriteString(goModToken(require.Version))
		if require.Indirect {
			content.WriteString(" // indirect")
		}
		content.WriteString("\n")
	}
	content.WriteString(")\n\n")
}

func writeWorkspaceReplaces(content *strings.Builder, replaces []workspaceReplaceDirective) {
	if len(replaces) == 0 {
		return
	}
	if content.Len() > 0 && !strings.HasSuffix(content.String(), "\n\n") {
		content.WriteString("\n")
	}
	content.WriteString("replace (\n")
	for _, replace := range replaces {
		content.WriteString("\t")
		content.WriteString(goModToken(replace.OldPath))
		if replace.OldVersion != "" {
			content.WriteString(" ")
			content.WriteString(goModToken(replace.OldVersion))
		}
		content.WriteString(" => ")
		content.WriteString(goModToken(replace.NewPath))
		if replace.NewVersion != "" {
			content.WriteString(" ")
			content.WriteString(goModToken(replace.NewVersion))
		}
		content.WriteString("\n")
	}
	content.WriteString(")\n")
}

func goModFields(line string) ([]string, error) {
	var fields []string
	for index := 0; index < len(line); {
		for index < len(line) && (line[index] == ' ' || line[index] == '\t' || line[index] == '\r') {
			index++
		}
		if index >= len(line) || strings.HasPrefix(line[index:], "//") {
			break
		}
		if line[index] == '"' || line[index] == '`' {
			start := index
			quote := line[index]
			index++
			for index < len(line) {
				if quote == '"' && line[index] == '\\' {
					index += 2
					continue
				}
				if line[index] == quote {
					index++
					break
				}
				index++
			}
			if index > len(line) || line[index-1] != quote {
				return nil, fmt.Errorf("unterminated quoted token")
			}
			value, err := strconv.Unquote(line[start:index])
			if err != nil {
				return nil, err
			}
			fields = append(fields, value)
			continue
		}
		start := index
		for index < len(line) && line[index] != ' ' && line[index] != '\t' && line[index] != '\r' {
			if strings.HasPrefix(line[index:], "//") {
				break
			}
			index++
		}
		if start != index {
			fields = append(fields, line[start:index])
		}
	}
	return fields, nil
}

func goModToken(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\r\n\"`") {
		return strconv.Quote(value)
	}
	return value
}

func copyWorkspaceGoSum(workDir, moduleRoot string) error {
	if moduleRoot == "" {
		return nil
	}
	source := filepath.Join(moduleRoot, "go.sum")
	info, err := os.Lstat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect app module go.sum %s: %w", source, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("app module go.sum %s is a symlink; symlink paths are not supported", source)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("app module go.sum %s is not a regular file", source)
	}
	return copyFile(source, filepath.Join(workDir, "go.sum"))
}
