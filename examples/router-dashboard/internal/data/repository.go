package data

import (
	"strings"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

const (
	issueDataKey    = "assets/data/issues.txt"
	issueMissingKey = "assets/data/missing-issues.txt"
)

var IssueRepositoryContext = gf.CreateContext(IssueRepository{})

type IssueProviderProps struct {
	Children []gf.Node
}

type ResourceStatusPanelProps struct{}

type IssueRepository struct {
	Resource        gf.Resource[[]Issue]
	Attempts        int
	Reload          func()
	Retry           func()
	SimulateFailure func()
}

func UseIssueRepository() IssueRepository {
	return gf.UseContext(IssueRepositoryContext)
}

func (repository IssueRepository) Issues() []Issue {
	if repository.Resource.Ready() {
		return repository.Resource.Value
	}
	return nil
}

func (repository IssueRepository) StatusText() string {
	switch {
	case repository.Resource.Ready():
		return "ready"
	case repository.Resource.Failed():
		return "failed"
	default:
		return "loading"
	}
}

func (repository IssueRepository) AttemptText() string {
	return gf.ToString(repository.Attempts)
}

func (repository IssueRepository) ErrorText() string {
	if repository.Resource.Err == nil {
		return "unknown resource error"
	}
	return repository.Resource.Err.Error()
}

func (repository IssueRepository) Ready() bool {
	return repository.Resource.Ready()
}

func (repository IssueRepository) Loading() bool {
	return repository.Resource.Loading()
}

func (repository IssueRepository) Failed() bool {
	return repository.Resource.Failed()
}

func (repository IssueRepository) HasData() bool {
	return repository.Resource.Ready() && len(repository.Resource.Value) > 0
}

func (repository IssueRepository) ReloadData() {
	if repository.Reload != nil {
		repository.Reload()
	}
}

func (repository IssueRepository) RetryData() {
	if repository.Retry != nil {
		repository.Retry()
	}
}

func (repository IssueRepository) SimulateDataFailure() {
	if repository.SimulateFailure != nil {
		repository.SimulateFailure()
	}
}

func StatusClass(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "rd-resource-status rd-resource-status-loading"
	}
	return "rd-resource-status rd-resource-status-" + status
}
