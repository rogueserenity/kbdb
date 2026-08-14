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
}

func TestHandleCreateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateKeyboardSuite))
}

func (s *HandleCreateKeyboardSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleCreateKeyboardSuite) TestSucceeds() {
	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: validKeyboardInput()})

	s.Require().NoError(err)
	s.Equal("Mode", out.Keyboard.Brand)
	s.NotEmpty(out.Keyboard.ID, "create must assign a server-generated id")
}

func (s *HandleCreateKeyboardSuite) TestBlankBrand_ReturnsError() {
	in := validKeyboardInput()
	in.Brand = "   "

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "brand must not be blank")
}

func (s *HandleCreateKeyboardSuite) TestInvalidVisibility_ReturnsError() {
	in := validKeyboardInput()
	in.Visibility = "everyone"

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "visibility")
}

func (s *HandleCreateKeyboardSuite) TestUnapprovedSize_ReturnsError() {
	size := "NotASize"
	in := validKeyboardInput()
	in.Size = &size

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "size")
	s.Require().ErrorContains(err, "not an approved")
}

// The cross-field rule: both values are individually approved, but the
// layout doesn't list this size among the ones it supports.
func (s *HandleCreateKeyboardSuite) TestLayoutNotValidForSize_ReturnsError() {
	size := "60%"
	layout := "MIT"
	in := validKeyboardInput()
	in.Size = &size
	in.Layout = &layout

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	// Both values are individually approved, so the generic wording would
	// claim the layout isn't an approved *size* - sending an agent to fix a
	// field that's already correct.
	s.Require().ErrorContains(err, `"MIT" is not a valid layout for size "60%"`)
	s.Require().NotContains(err.Error(), "approved keyboard_size")
}

func (s *HandleCreateKeyboardSuite) TestLayoutValidForSize_Succeeds() {
	size := "60%"
	layout := "WKL"
	in := validKeyboardInput()
	in.Size = &size
	in.Layout = &layout

	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().NoError(err)
}

func (s *HandleCreateKeyboardSuite) TestAlreadyExists_ReturnsAlreadyExists() {
	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: validKeyboardInput()})

	s.Require().ErrorIs(err, errKeyboardAlreadyExists)
}

type HandleUpdateKeyboardSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
}

func TestHandleUpdateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateKeyboardSuite))
}

func (s *HandleUpdateKeyboardSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleUpdateKeyboardSuite) TestSucceeds() {
	s.mockKeyboards.EXPECT().
		Update(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleUpdateKeyboard(s.mockKeyboards)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{
		KeyboardID:    "kb-1",
		KeyboardInput: validKeyboardInput(),
	})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Keyboard.ID, "update must target the requested id")
}

func (s *HandleUpdateKeyboardSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleUpdateKeyboard(s.mockKeyboards)
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

	handler := handleUpdateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{
		KeyboardID:    "missing",
		KeyboardInput: validKeyboardInput(),
	})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

type HandleDeleteKeyboardSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
	mockBuilds    *mocks.MockBuildRepository
	mockImages    *mocks.MockBuildImageStore
}

func TestHandleDeleteKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeyboardSuite))
}

func (s *HandleDeleteKeyboardSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
}

func (s *HandleDeleteKeyboardSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeyboardSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: ""})

	s.Require().Error(err)
}

func (s *HandleDeleteKeyboardSuite) TestInvalidOnDelete_ReturnsError() {
	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "bogus"})

	s.Require().Error(err)
}

func (s *HandleDeleteKeyboardSuite) TestBlock_Referenced_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return([]string{"build-1"}, nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "block"})

	s.Require().ErrorContains(err, "build-1")
}

func (s *HandleDeleteKeyboardSuite) TestDetach_Referenced_Succeeds_DoesNotCheckReferences() {
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "detach"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeyboardSuite) TestCascade_Referenced_ReturnsDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil, nil)
	s.mockImages.EXPECT().
		BestEffortDelete(mock.Anything, mock.Anything).
		Return()
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"build-1"}, out.DeletedBuildIDs)
}

func (s *HandleDeleteKeyboardSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(errors.New("delete failed"))

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().Error(err)
}

// REST rejects a malformed date via api/openapi.yaml's `format: date`. MCP
// has no such validator, and nothing downstream re-checks: a bad date
// reaches DynamoDB and every later REST read of that row fails its date
// parse with a 500, unrepairable through the API.
func (s *HandleCreateKeyboardSuite) TestMalformedOrderDate_ReturnsError() {
	bad := "next tuesday"
	in := validKeyboardInput()
	in.Purchase = &schema.KeyboardPurchase{OrderDate: &bad}

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "purchase.order_date")
	s.Require().ErrorContains(err, "YYYY-MM-DD")
}

func (s *HandleCreateKeyboardSuite) TestMalformedDeliveryDate_ReturnsError() {
	bad := "2026-13-45"
	in := validKeyboardInput()
	in.Purchase = &schema.KeyboardPurchase{DeliveryDate: &bad}

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "purchase.delivery_date")
}

func (s *HandleCreateKeyboardSuite) TestWellFormedDates_Succeed() {
	ordered, delivered := "2026-01-15", "2026-02-01"
	in := validKeyboardInput()
	in.Purchase = &schema.KeyboardPurchase{OrderDate: &ordered, DeliveryDate: &delivered}

	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleCreateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().NoError(err)
}

func (s *HandleUpdateKeyboardSuite) TestMalformedOrderDate_ReturnsError() {
	bad := "01/15/2026"
	in := validKeyboardInput()
	in.Purchase = &schema.KeyboardPurchase{OrderDate: &bad}

	handler := handleUpdateKeyboard(s.mockKeyboards)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{
		KeyboardID:    "kb-1",
		KeyboardInput: in,
	})

	s.Require().ErrorContains(err, "purchase.order_date")
}
