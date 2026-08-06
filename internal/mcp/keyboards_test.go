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

type HandleListKeyboardsSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeyboardRepository
}

func TestHandleListKeyboardsSuite(t *testing.T) {
	suite.Run(t, new(HandleListKeyboardsSuite))
}

func (s *HandleListKeyboardsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleListKeyboardsSuite) TestBlankUserID_DefaultsToCaller() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, defaultListLimit, "").
		Return([]repository.Keyboard{{ID: "kb-1", Brand: "Mode", Name: "Sixty"}}, "", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Keyboards, 1)
	s.Equal("kb-1", out.Keyboards[0].ID)
}

func (s *HandleListKeyboardsSuite) TestOwnCollection_ReadsAllVisibilityTiers() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
			repository.VisibilityPrivate,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().NoError(err)
}

func (s *HandleListKeyboardsSuite) TestOtherUsersCollection_ExcludesPrivate() {
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{UserID: otherID})

	s.Require().NoError(err)
}

func (s *HandleListKeyboardsSuite) TestLimitAboveMax_IsClamped() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, maxListLimit, mock.Anything).
		Return(nil, "", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{Limit: 5000})

	s.Require().NoError(err)
}

func (s *HandleListKeyboardsSuite) TestCursorIsPropagated() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "page-2").
		Return(nil, "page-3", nil)

	handler := handleListKeyboards(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{Cursor: "page-2"})

	s.Require().NoError(err)
	s.Equal("page-3", out.NextCursor)
}

func (s *HandleListKeyboardsSuite) TestNoCallerIdentity_ReturnsError() {
	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.ListKeyboardsInput{})

	s.Require().ErrorIs(err, errNoCallerIdentity)
}

func (s *HandleListKeyboardsSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", errors.New("query failed"))

	handler := handleListKeyboards(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().ErrorContains(err, "failed to list keyboards")
}

type HandleGetKeyboardSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeyboardRepository
}

func TestHandleGetKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleGetKeyboardSuite))
}

func (s *HandleGetKeyboardSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleGetKeyboardSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{
			ID:         "kb-1",
			Brand:      "Mode",
			Name:       "Sixty",
			Visibility: repository.VisibilityPrivate,
		}, nil)

	handler := handleGetKeyboard(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1"})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Keyboard.ID)
	s.Equal("Mode", out.Keyboard.Brand)
}

func (s *HandleGetKeyboardSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleGetKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "  "})

	s.Require().ErrorContains(err, "keyboard_id must not be blank")
}

func (s *HandleGetKeyboardSuite) TestNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "missing"})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

func (s *HandleGetKeyboardSuite) TestOtherUsersPrivateKeyboard_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := handleGetKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1", UserID: otherID})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

func (s *HandleGetKeyboardSuite) TestOtherUsersPublicKeyboard_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "kb-1").
		Return(&repository.Keyboard{
			ID:         "kb-1",
			Brand:      "Mode",
			Visibility: repository.VisibilityPublic,
		}, nil)

	handler := handleGetKeyboard(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1", UserID: otherID})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Keyboard.ID)
}

func (s *HandleGetKeyboardSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("get failed"))

	handler := handleGetKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1"})

	s.Require().ErrorContains(err, "failed to get keyboard")
}

func validKeyboardInput() schema.KeyboardInput {
	return schema.KeyboardInput{
		Brand:      "Mode",
		Name:       "Sixty",
		Visibility: "private",
	}
}

type HandleCreateKeyboardSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
	mockLookups   *mocks.MockLookupRepository
}

func TestHandleCreateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateKeyboardSuite))
}

func (s *HandleCreateKeyboardSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockLookups = mocks.NewMockLookupRepository(s.T())
}

func (s *HandleCreateKeyboardSuite) TestSucceeds() {
	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockLookups)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: validKeyboardInput()})

	s.Require().NoError(err)
	s.Equal("Mode", out.Keyboard.Brand)
	s.NotEmpty(out.Keyboard.ID, "create must assign a server-generated id")
}

func (s *HandleCreateKeyboardSuite) TestBlankBrand_ReturnsError() {
	in := validKeyboardInput()
	in.Brand = "   "

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockLookups)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "brand must not be blank")
}

func (s *HandleCreateKeyboardSuite) TestInvalidVisibility_ReturnsError() {
	in := validKeyboardInput()
	in.Visibility = "everyone"

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockLookups)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "visibility")
}

func (s *HandleCreateKeyboardSuite) TestUnapprovedSize_ReturnsError() {
	size := "NotASize"
	in := validKeyboardInput()
	in.Size = &size

	s.mockLookups.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardSize).
		Return(&repository.Lookup{Values: []any{"60%"}}, nil)

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockLookups)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "size")
	s.Require().ErrorContains(err, "not an approved")
}

// The cross-field rule: both values are individually approved, but the
// layout doesn't list this size among the ones it supports.
func (s *HandleCreateKeyboardSuite) TestLayoutNotValidForSize_ReturnsError() {
	size := "60%"
	layout := "WKL"
	in := validKeyboardInput()
	in.Size = &size
	in.Layout = &layout

	s.mockLookups.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardSize).
		Return(&repository.Lookup{Values: []any{"60%"}}, nil)
	s.mockLookups.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(&repository.Lookup{Values: []any{
			map[string]any{"name": "WKL", "sizes": []any{"TKL"}},
		}}, nil)

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockLookups)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "layout")
}

func (s *HandleCreateKeyboardSuite) TestLayoutValidForSize_Succeeds() {
	size := "60%"
	layout := "WKL"
	in := validKeyboardInput()
	in.Size = &size
	in.Layout = &layout

	s.mockLookups.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardSize).
		Return(&repository.Lookup{Values: []any{"60%"}}, nil)
	s.mockLookups.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(&repository.Lookup{Values: []any{
			map[string]any{"name": "WKL", "sizes": []any{"60%", "TKL"}},
		}}, nil)
	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockLookups)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().NoError(err)
}

func (s *HandleCreateKeyboardSuite) TestAlreadyExists_ReturnsAlreadyExists() {
	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockLookups)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: validKeyboardInput()})

	s.Require().ErrorIs(err, errKeyboardAlreadyExists)
}

type HandleUpdateKeyboardSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
	mockLookups   *mocks.MockLookupRepository
}

func TestHandleUpdateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateKeyboardSuite))
}

func (s *HandleUpdateKeyboardSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockLookups = mocks.NewMockLookupRepository(s.T())
}

func (s *HandleUpdateKeyboardSuite) TestSucceeds() {
	s.mockKeyboards.EXPECT().
		Update(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleUpdateKeyboard(s.mockKeyboards, s.mockLookups)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{
		KeyboardID:    "kb-1",
		KeyboardInput: validKeyboardInput(),
	})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Keyboard.ID, "update must target the requested id")
}

func (s *HandleUpdateKeyboardSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleUpdateKeyboard(s.mockKeyboards, s.mockLookups)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{
		KeyboardID:    "  ",
		KeyboardInput: validKeyboardInput(),
	})

	s.Require().ErrorContains(err, "keyboard_id must not be blank")
}

func (s *HandleUpdateKeyboardSuite) TestNotFound_ReturnsNotFound() {
	s.mockKeyboards.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleUpdateKeyboard(s.mockKeyboards, s.mockLookups)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{
		KeyboardID:    "missing",
		KeyboardInput: validKeyboardInput(),
	})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

type HandleDeleteKeyboardSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeyboardRepository
}

func TestHandleDeleteKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeyboardSuite))
}

func (s *HandleDeleteKeyboardSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleDeleteKeyboardSuite) TestSucceeds() {
	s.mockRepo.EXPECT().Delete(mock.Anything, "kb-1").Return(nil)

	handler := handleDeleteKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeyboardSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleDeleteKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: ""})

	s.Require().ErrorContains(err, "keyboard_id must not be blank")
}

func (s *HandleDeleteKeyboardSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().Delete(mock.Anything, mock.Anything).Return(errors.New("delete failed"))

	handler := handleDeleteKeyboard(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().ErrorContains(err, "failed to delete keyboard")
}
