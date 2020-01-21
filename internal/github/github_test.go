package github

import (
	"github.com/flow-lab/flow/internal/mocks"
	"github.com/golang/mock/gomock"
	"github.com/google/go-github/v27/github"

	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate mockgen -source=github.go -destination=../mocks/mock_github.go -package=mocks

func TestCreateRepository(t *testing.T) {
	t.Run("Should create a service", func(t *testing.T) {
		s := NewFlowGithubService("test", "test")

		assert.NotNil(t, s)
	})

	t.Run("Should create a repo", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ghs := mocks.NewMockmimiroGithubClient(ctrl)

		repository := &github.Repository{
			Name:    github.String("test-repo-name"),
			Private: github.Bool(true),
		}
		ghs.EXPECT().
			RepositoriesCreate(gomock.Any(), gomock.Eq("TINE-SA"), repository).
			Return(&github.Repository{
				Name:     github.String("test-repo-name"),
				Private:  github.Bool(true),
				CloneURL: github.String("https://clone-url"),
			}, nil, nil).
			Times(1)

		testTeam := &github.Team{Name: github.String("test-team"), ID: github.Int64(0)}
		ghs.EXPECT().
			TeamsGetTeamBySlug(gomock.Any(), gomock.Eq("TINE-SA"), gomock.Any()).
			Return(testTeam, nil, nil).
			Times(3)

		ghs.EXPECT().
			TeamsAddTeamRepo(gomock.Any(), gomock.Eq(testTeam.GetID()), gomock.Eq("TINE-SA"), gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(3)

		s := &flowGithubService{
			org: "TINE-SA",
			mgc: ghs,
		}

		cloneUrl, err := s.CreateRepository(context.TODO(), "test-repo-name")

		assert.Nil(t, err)
		assert.NotNil(t, cloneUrl)
	})

	t.Run("Should archive a repo", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ghs := mocks.NewMockmimiroGithubClient(ctrl)

		repository := &github.Repository{
			Name:    github.String("test-repo-name"),
			Private: github.Bool(true),
		}
		ghs.EXPECT().
			RepositoriesGetRepo(gomock.Any(), gomock.Eq("TINE-SA"), *repository.Name).
			Return(&github.Repository{
				Name:     github.String("test-repo-name"),
				Private:  github.Bool(true),
				CloneURL: github.String("https://clone-url"),
			}, nil, nil).
			Times(1)

		ghs.EXPECT().
			RepositoriesEditRepo(gomock.Any(), gomock.Eq("TINE-SA"), *repository.Name, gomock.Any()).
			Return(&github.Repository{
				Name:     github.String("test-repo-name"),
				Private:  github.Bool(true),
				CloneURL: github.String("https://clone-url"),
				Archived: github.Bool(true),
			}, nil, nil).
			Times(1)

		s := &flowGithubService{
			org: "TINE-SA",
			mgc: ghs,
		}

		err := s.ArchiveRepository(context.TODO(), "test-repo-name")

		assert.Nil(t, err)
	})

	t.Run("Should rename a repo", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ghs := mocks.NewMockmimiroGithubClient(ctrl)

		repository := &github.Repository{
			Name:    github.String("test-repo-name"),
			Private: github.Bool(true),
		}
		ghs.EXPECT().
			RepositoriesGetRepo(gomock.Any(), gomock.Eq("TINE-SA"), *repository.Name).
			Return(&github.Repository{
				Name:     github.String("test-repo-name"),
				Private:  github.Bool(true),
				CloneURL: github.String("https://clone-url"),
			}, nil, nil).
			Times(1)

		ghs.EXPECT().
			RepositoriesEditRepo(gomock.Any(), gomock.Eq("TINE-SA"), *repository.Name, gomock.Any()).
			Return(&github.Repository{
				Name:     github.String("test-repo-name"),
				Private:  github.Bool(true),
				CloneURL: github.String("https://clone-url"),
				Archived: github.Bool(true),
			}, nil, nil).
			Times(1)

		s := &flowGithubService{
			org: "TINE-SA",
			mgc: ghs,
		}

		err := s.Rename(context.TODO(), "test-repo-name", "new-repo-name")

		assert.Nil(t, err)
	})
}
