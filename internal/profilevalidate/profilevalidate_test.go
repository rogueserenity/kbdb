package profilevalidate_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/profilevalidate"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type ValidateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(ValidateSuite))
}

func valid() repository.Profile {
	return repository.Profile{Username: "alice_kb"}
}

func names(errs []profilevalidate.FieldError) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Name
	}
	return out
}

func (s *ValidateSuite) TestValidMinimal_NoErrors() {
	s.Empty(profilevalidate.Validate(valid()))
}

func (s *ValidateSuite) TestValidFull_NoErrors() {
	discord := "alice_kb"
	bio := "keebs enjoyer"
	p := repository.Profile{
		Username:        "alice-99",
		Discoverable:    true,
		DiscordUsername: &discord,
		Bio:             &bio,
		Links: []repository.ProfileLink{
			{Name: "Twitch", URL: "https://twitch.tv/alice"},
			{Name: "site", URL: "https://example.com/path?q=1"},
		},
	}

	s.Empty(profilevalidate.Validate(p))
}

func (s *ValidateSuite) TestUsernameTooShort_Flagged() {
	p := valid()
	p.Username = "ab"

	s.Equal([]string{"username"}, names(profilevalidate.Validate(p)))
}

func (s *ValidateSuite) TestUsernameUppercase_Flagged() {
	p := valid()
	p.Username = "Alice"

	s.Equal([]string{"username"}, names(profilevalidate.Validate(p)))
}

func (s *ValidateSuite) TestUsernameUserPrefix_Flagged() {
	p := valid()
	p.Username = "user-alice"

	errs := profilevalidate.Validate(p)
	s.Require().Len(errs, 1)
	s.Equal("username", errs[0].Name)
	s.Contains(errs[0].Reason, `"user-"`)
}

func (s *ValidateSuite) TestUsernameUnderscoreOnlyPrefix_NotFlaggedAsUserBan() {
	// "users" contains "user" but not the "user-" prefix.
	p := valid()
	p.Username = "users_r_us"

	s.Empty(profilevalidate.Validate(p))
}

func (s *ValidateSuite) TestDiscordUsernameOver32Runes_Flagged() {
	p := valid()
	long := strings.Repeat("x", 33)
	p.DiscordUsername = &long

	s.Equal([]string{"discord_username"}, names(profilevalidate.Validate(p)))
}

func (s *ValidateSuite) TestDiscordUsernameExactly32Runes_OK() {
	p := valid()
	exact := strings.Repeat("x", 32)
	p.DiscordUsername = &exact

	s.Empty(profilevalidate.Validate(p))
}

func (s *ValidateSuite) TestBioOver500Runes_Flagged() {
	p := valid()
	long := strings.Repeat("é", 501)
	p.Bio = &long

	s.Equal([]string{"bio"}, names(profilevalidate.Validate(p)))
}

func (s *ValidateSuite) TestSixthLink_Flagged() {
	p := valid()
	for range 6 {
		p.Links = append(p.Links, repository.ProfileLink{Name: "x", URL: "https://x.example"})
	}

	s.Contains(names(profilevalidate.Validate(p)), "links")
}

func (s *ValidateSuite) TestLinkNameBlank_Flagged() {
	p := valid()
	p.Links = []repository.ProfileLink{{Name: "  ", URL: "https://x.example"}}

	s.Equal([]string{"links[0].name"}, names(profilevalidate.Validate(p)))
}

func (s *ValidateSuite) TestLinkNameOver32_Flagged() {
	p := valid()
	p.Links = []repository.ProfileLink{{Name: strings.Repeat("n", 33), URL: "https://x.example"}}

	s.Equal([]string{"links[0].name"}, names(profilevalidate.Validate(p)))
}

func (s *ValidateSuite) TestLinkURLHTTP_Flagged() {
	p := valid()
	p.Links = []repository.ProfileLink{{Name: "site", URL: "http://x.example"}}

	errs := profilevalidate.Validate(p)
	s.Require().Len(errs, 1)
	s.Equal("links[0].url", errs[0].Name)
	s.Contains(errs[0].Reason, "https")
}

func (s *ValidateSuite) TestLinkURLNoHost_Flagged() {
	p := valid()
	p.Links = []repository.ProfileLink{{Name: "site", URL: "https:///path"}}

	s.Equal([]string{"links[0].url"}, names(profilevalidate.Validate(p)))
}

func (s *ValidateSuite) TestLinkURLBlank_Flagged() {
	p := valid()
	p.Links = []repository.ProfileLink{{Name: "site", URL: ""}}

	s.Equal([]string{"links[0].url"}, names(profilevalidate.Validate(p)))
}

func (s *ValidateSuite) TestNormalize_BlankDiscordUsername_SetToNil() {
	blank := ""
	p := repository.Profile{Username: "alice_kb", DiscordUsername: &blank}

	profilevalidate.Normalize(&p)

	s.Nil(p.DiscordUsername)
}

func (s *ValidateSuite) TestNormalize_WhitespaceDiscordUsername_SetToNil() {
	whitespace := "   "
	p := repository.Profile{Username: "alice_kb", DiscordUsername: &whitespace}

	profilevalidate.Normalize(&p)

	s.Nil(p.DiscordUsername)
}

func (s *ValidateSuite) TestNormalize_NonBlankDiscordUsername_Unchanged() {
	discord := "alice_kb"
	p := repository.Profile{Username: "alice_kb", DiscordUsername: &discord}

	profilevalidate.Normalize(&p)

	s.Require().NotNil(p.DiscordUsername)
	s.Equal("alice_kb", *p.DiscordUsername)
}

func (s *ValidateSuite) TestNormalize_NilDiscordUsername_Unchanged() {
	p := repository.Profile{Username: "alice_kb"}

	profilevalidate.Normalize(&p)

	s.Nil(p.DiscordUsername)
}

func (s *ValidateSuite) TestSecondLinkIndexInFieldName() {
	p := valid()
	p.Links = []repository.ProfileLink{
		{Name: "ok", URL: "https://ok.example"},
		{Name: "bad", URL: "ftp://nope.example"},
	}

	s.Equal([]string{"links[1].url"}, names(profilevalidate.Validate(p)))
}
