package repoapi

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type ProfileMapperSuite struct {
	suite.Suite
}

func TestProfileMapperSuite(t *testing.T) {
	suite.Run(t, new(ProfileMapperSuite))
}

func profileImageKeyPtr(s string) *repository.ProfileImageKey {
	k := repository.ProfileImageKey(s)
	return &k
}

func (s *ProfileMapperSuite) TestProfileToAPI_FullProfile_NoAvatar() {
	p := repository.Profile{
		OwnerID:         "user-alice",
		Username:        "alice",
		Discoverable:    true,
		DiscordUsername: strPtr("alice_kb"),
		Bio:             strPtr("keebs"),
		Links: []repository.ProfileLink{
			{Name: "Twitch", URL: "https://twitch.tv/alice"},
			{Name: "Insta", URL: "https://instagram.com/alice"},
		},
	}

	out, err := ProfileToAPI(s.T().Context(), p, mocks.NewMockProfileImageStore(s.T()))

	s.Require().NoError(err)
	s.Equal("alice", out.Username)
	s.Require().NotNil(out.UserId)
	s.Equal("user-alice", *out.UserId)
	s.Require().NotNil(out.Discoverable)
	s.True(*out.Discoverable)
	s.Require().NotNil(out.DiscordUsername)
	s.Equal("alice_kb", *out.DiscordUsername)
	s.Require().NotNil(out.Bio)
	s.Equal("keebs", *out.Bio)
	s.Require().NotNil(out.Links)
	s.Len(*out.Links, 2)
	s.Equal(api.ProfileLink{Name: "Twitch", Url: "https://twitch.tv/alice"}, (*out.Links)[0])
	s.Nil(out.Avatar)
}

func (s *ProfileMapperSuite) TestProfileToAPI_ExposesSubjectAsUserID() {
	// user_id is on the single-profile response so callers can address the
	// {userId}-keyed collection routes.
	out, err := ProfileToAPI(s.T().Context(), repository.Profile{
		OwnerID:  "user-alice",
		Username: "alice",
	}, mocks.NewMockProfileImageStore(s.T()))

	s.Require().NoError(err)
	s.Equal("alice", out.Username)
	s.Require().NotNil(out.UserId)
	s.Equal("user-alice", *out.UserId)
}

func (s *ProfileMapperSuite) TestProfileToAPI_PresignsAvatar() {
	images := mocks.NewMockProfileImageStore(s.T())
	images.EXPECT().
		PresignGet(mock.Anything, repository.ProfileImageKey("profiles/user-alice/avatar")).
		Return("https://example.com/avatar", nil)

	p := repository.Profile{
		Username:   "alice",
		AvatarPath: profileImageKeyPtr("profiles/user-alice/avatar"),
	}

	out, err := ProfileToAPI(s.T().Context(), p, images)

	s.Require().NoError(err)
	s.Require().NotNil(out.Avatar)
	s.Equal("https://example.com/avatar", out.Avatar.Url)
}

func (s *ProfileMapperSuite) TestProfileToAPI_PresignError_Propagates() {
	images := mocks.NewMockProfileImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, mock.Anything).Return("", errors.New("s3 down"))

	_, err := ProfileToAPI(s.T().Context(), repository.Profile{
		Username:   "alice",
		AvatarPath: profileImageKeyPtr("profiles/user-alice/avatar"),
	}, images)

	s.Require().Error(err)
}

func (s *ProfileMapperSuite) TestProfileToAPI_EmptyLinks_OmittedNotEmptySlice() {
	out, err := ProfileToAPI(s.T().Context(), repository.Profile{Username: "alice"},
		mocks.NewMockProfileImageStore(s.T()))

	s.Require().NoError(err)
	s.Nil(out.Links)
}

func (s *ProfileMapperSuite) TestProfileToRepo_MapsBodyFields_NotAvatarOrDerived() {
	in := api.ProfileInput{
		Username:        "alice",
		Discoverable:    boolPtr(true),
		DiscordUsername: strPtr("alice_kb"),
		Bio:             strPtr("keebs"),
		Links:           &[]api.ProfileLink{{Name: "Twitch", Url: "https://twitch.tv/alice"}},
	}

	p := ProfileToRepo(in)

	s.Equal("alice", p.Username)
	s.True(p.Discoverable)
	s.Require().NotNil(p.DiscordUsername)
	s.Equal("alice_kb", *p.DiscordUsername)
	s.Require().Len(p.Links, 1)
	s.Equal(repository.ProfileLink{Name: "Twitch", URL: "https://twitch.tv/alice"}, p.Links[0])
	// Not set from the body:
	s.Empty(p.OwnerID)
	s.Nil(p.AvatarPath)
	s.Nil(p.DiscoverablePK)
	s.Nil(p.DiscordUsernameLC)
}

func (s *ProfileMapperSuite) TestProfileToRepo_DiscoverableOmitted_DefaultsFalse() {
	p := ProfileToRepo(api.ProfileInput{Username: "alice"})

	s.False(p.Discoverable)
}
