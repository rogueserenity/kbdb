package repomcp

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository"
)

type ProfileMapperSuite struct {
	suite.Suite
}

func TestProfileMapperSuite(t *testing.T) {
	suite.Run(t, new(ProfileMapperSuite))
}

func (s *ProfileMapperSuite) TestProfileToMCP_FullProfile() {
	discord := "alice_kb"
	bio := "keebs"
	p := repository.Profile{
		StytchUserID:    "user-alice",
		Username:        "alice",
		Discoverable:    true,
		DiscordUsername: &discord,
		Bio:             &bio,
		Links: []repository.ProfileLink{
			{Name: "Twitch", URL: "https://twitch.tv/alice"},
		},
	}

	out := ProfileToMCP(p)

	s.Equal("alice", out.Username)
	s.True(out.Discoverable)
	s.Require().NotNil(out.DiscordUsername)
	s.Equal("alice_kb", *out.DiscordUsername)
	s.Require().NotNil(out.Bio)
	s.Equal("keebs", *out.Bio)
	s.Require().Len(out.Links, 1)
	s.Equal("Twitch", out.Links[0].Name)
	s.Equal("https://twitch.tv/alice", out.Links[0].URL)
	s.False(out.HasAvatar)
}

func (s *ProfileMapperSuite) TestProfileToMCP_AvatarReportedAsBool() {
	key := repository.ProfileImageKey("profiles/user-alice/avatar")
	out := ProfileToMCP(repository.Profile{Username: "alice", AvatarPath: &key})

	s.True(out.HasAvatar)
}

func (s *ProfileMapperSuite) TestProfileToMCP_EmptyLinks_Nil() {
	out := ProfileToMCP(repository.Profile{Username: "alice"})

	s.Nil(out.Links)
}
