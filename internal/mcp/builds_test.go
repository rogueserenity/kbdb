package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

func validBuildInput() schema.BuildInput {
	return schema.BuildInput{
		Keyboard:   "kb-1",
		Visibility: "private",
	}
}

type HandleCreateBuildSuite struct {
	suite.Suite

	mockBuilds *mocks.MockBuildRepository
}

func TestHandleCreateBuildSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateBuildSuite))
}

func (s *HandleCreateBuildSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
}

func (s *HandleCreateBuildSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, b repository.Build) (*repository.Build, error) {
			return &b, nil
		})

	handler := handleCreateBuild(s.mockBuilds)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Build.Keyboard)
	s.NotEmpty(out.Build.ID, "create must assign a server-generated id")
}

func (s *HandleCreateBuildSuite) TestBlankKeyboard_ReturnsError() {
	in := validBuildInput()
	in.Keyboard = "   "

	handler := handleCreateBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "keyboard must not be blank")
}

func (s *HandleCreateBuildSuite) TestInvalidVisibility_ReturnsError() {
	in := validBuildInput()
	in.Visibility = "everyone"

	handler := handleCreateBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "visibility")
}

func (s *HandleCreateBuildSuite) TestMalformedBuildDate_ReturnsError() {
	in := validBuildInput()
	badDate := "not-a-date"
	in.BuildDate = &badDate

	handler := handleCreateBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "build_date")
}

func (s *HandleCreateBuildSuite) TestUnapprovedStabsName_ReturnsError() {
	in := validBuildInput()
	in.Stabs = &schema.BuildStabs{Name: strPtrMCP("NotApproved")}

	handler := handleCreateBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "stabs.name")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateBuildSuite) TestUnapprovedCaseMountType_ReturnsError() {
	in := validBuildInput()
	in.CaseMountType = &schema.BuildCaseMountType{Type: strPtrMCP("NotApproved")}

	handler := handleCreateBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "case_mount_type.type")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateBuildSuite) TestAlreadyExists_ReturnsAlreadyExists() {
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	handler := handleCreateBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorIs(err, errBuildAlreadyExists)
}

func (s *HandleCreateBuildSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("create failed"))

	handler := handleCreateBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorContains(err, "failed to create build")
}

func strPtrMCP(s string) *string { return &s }
