package github

import (
	"context"
	"github.com/google/go-github/v28/github"
	"golang.org/x/oauth2"
)

// FlowGithubService wraps the Github client.
type FlowGithubService interface {
	// CreateRepository creates repository and configure teams
	CreateRepository(ctx context.Context, repo string) (cloneUrl *string, err error)
	// ArchiveRepository archives repository
	ArchiveRepository(ctx context.Context, repo string) error
	// Rename repository
	Rename(ctx context.Context, repo string, newRepo string) error
}

type flowGithubService struct {
	org string
	mgc mimiroGithubClient
}

// CreateRepository creates repository and add it to correct teams
func (m *flowGithubService) CreateRepository(ctx context.Context, repo string) (*string, error) {
	repository, _, _ := m.mgc.RepositoriesCreate(ctx, m.org, &github.Repository{
		Private: github.Bool(true),
		Name:    github.String(repo),
	})

	// TODO [grokrz]: implement configuration

	return repository.CloneURL, nil
}

func (m *flowGithubService) ArchiveRepository(ctx context.Context, repo string) error {
	repository, _, err := m.mgc.RepositoriesGetRepo(ctx, m.org, repo)
	if err != nil {
		return err
	}

	repository.Archived = github.Bool(true)
	_, _, err = m.mgc.RepositoriesEditRepo(ctx, m.org, repo, repository)
	return err
}

func (m *flowGithubService) Rename(ctx context.Context, repo string, newRepo string) error {
	repository, _, err := m.mgc.RepositoriesGetRepo(ctx, m.org, repo)
	if err != nil {
		return err
	}

	repository.Name = github.String(newRepo)
	_, _, err = m.mgc.RepositoriesEditRepo(ctx, m.org, repo, repository)
	return err
}

// NewFlowGithubService create new instance of service
func NewFlowGithubService(org string, accessToken string) FlowGithubService {
	if org == "" {
		panic("org is required")
	}
	if accessToken == "" {
		panic("accessToken is required")
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	mgc := mimiroGithubClientWrapper{client}
	return &flowGithubService{
		org: org,
		mgc: &mgc,
	}
}

// mimiroGithubClient wraps github client so it can be easily tested.
type mimiroGithubClient interface {
	// RepositoriesCreate delegates to github.Repositories.Create.
	RepositoriesCreate(ctx context.Context, org string, repo *github.Repository) (*github.Repository, *github.Response, error)
	// GetTeamBySlug delegates to github.Teams.GetTeamBySlug
	TeamsGetTeamBySlug(ctx context.Context, org, slug string) (*github.Team, *github.Response, error)
	// TeamsAddTeamRepo delegates to github.Teams.AddTeamRepo.
	TeamsAddTeamRepo(ctx context.Context, team int64, owner string, repo string, opt *github.TeamAddTeamRepoOptions) (*github.Response, error)
	// RepositoriesGetRepo gets repo
	RepositoriesGetRepo(ctx context.Context, org, repo string) (*github.Repository, *github.Response, error)
	// RepositoriesEditRepo edits repo
	RepositoriesEditRepo(ctx context.Context, org, repo string, repository *github.Repository) (*github.Repository, *github.Response, error)
}

type mimiroGithubClientWrapper struct {
	*github.Client
}

func (m *mimiroGithubClientWrapper) RepositoriesCreate(ctx context.Context, org string, repo *github.Repository) (*github.Repository, *github.Response, error) {
	return m.Repositories.Create(ctx, org, repo)
}

func (m *mimiroGithubClientWrapper) RepositoriesGetRepo(ctx context.Context, org, repo string) (*github.Repository, *github.Response, error) {
	return m.Repositories.Get(ctx, org, repo)
}

func (m *mimiroGithubClientWrapper) RepositoriesEditRepo(ctx context.Context, org, repo string, repository *github.Repository) (*github.Repository, *github.Response, error) {
	return m.Repositories.Edit(ctx, org, repo, repository)
}

func (m *mimiroGithubClientWrapper) TeamsGetTeamBySlug(ctx context.Context, org, slug string) (*github.Team, *github.Response, error) {
	return m.Teams.GetTeamBySlug(ctx, org, slug)
}

func (m *mimiroGithubClientWrapper) TeamsAddTeamRepo(ctx context.Context, team int64, owner string, repo string, opt *github.TeamAddTeamRepoOptions) (*github.Response, error) {
	return m.Teams.AddTeamRepo(ctx, team, owner, repo, opt)
}
