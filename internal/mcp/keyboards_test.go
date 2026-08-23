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

	s.Require().ErrorIs(err, errMutationNotFound)
}

type HandleDeleteKeyboardSuite struct {
	suite.Suite

	mockKeyboards      *mocks.MockKeyboardRepository
	mockBuilds         *mocks.MockBuildRepository
	mockBuildImages    *mocks.MockBuildImageStore
	mockKeyboardImages *mocks.MockKeyboardImageStore
}

func TestHandleDeleteKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeyboardSuite))
}

func (s *HandleDeleteKeyboardSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImages = mocks.NewMockBuildImageStore(s.T())
	s.mockKeyboardImages = mocks.NewMockKeyboardImageStore(s.T())
}

func (s *HandleDeleteKeyboardSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeyboardSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: ""})

	s.Require().Error(err)
}

func (s *HandleDeleteKeyboardSuite) TestInvalidOnDelete_ReturnsError() {
	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "bogus"})

	s.Require().Error(err)
}

func (s *HandleDeleteKeyboardSuite) TestBlock_Referenced_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return([]string{"build-1"}, nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "block"})

	s.Require().ErrorContains(err, "build-1")
}

func (s *HandleDeleteKeyboardSuite) TestDetach_Referenced_Succeeds_DoesNotCheckReferences() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "detach"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeyboardSuite) TestCascade_Referenced_ReturnsDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"build-1"}, out.DeletedBuildIDs)
}

func (s *HandleDeleteKeyboardSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(errors.New("delete failed"))

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().Error(err)
}

func (s *HandleDeleteKeyboardSuite) TestImageDeleteFails_ReturnsError_DoesNotDeleteKeyboard() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Images: []repository.KeyboardImage{
			{ImageID: "img-1", Path: "keyboards/alice/kb-1/images/img-1"},
		}}, nil)
	s.mockKeyboardImages.EXPECT().
		DeleteKeyboardImage(mock.Anything, repository.KeyboardImageKey("keyboards/alice/kb-1/images/img-1")).
		Return(errors.New("s3 unavailable"))

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().Error(err)
	// mockKeyboards has no .EXPECT() for Delete - verifies the DB record
	// was never touched.
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

type HandleGetKeyboardImageURLSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeyboardRepository
	mockImages *mocks.MockKeyboardImageStore
}

func TestHandleGetKeyboardImageURLSuite(t *testing.T) {
	suite.Run(t, new(HandleGetKeyboardImageURLSuite))
}

func (s *HandleGetKeyboardImageURLSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
}

func (s *HandleGetKeyboardImageURLSuite) TestSucceeds() {
	imagePath := repository.KeyboardImageKey("keyboards/caller-0001/kb-1/images/img-1")
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{
			ID:         "kb-1",
			Visibility: repository.VisibilityPrivate,
			Images:     []repository.KeyboardImage{{ImageID: "img-1", Path: imagePath}},
		}, nil)
	s.mockImages.EXPECT().
		PresignGetKeyboardImage(mock.Anything, imagePath).
		Return("https://example.com/presigned", nil)

	handler := handleGetKeyboardImageURL(s.mockRepo, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardImageURLInput{
		KeyboardID: "kb-1",
		ImageID:    "img-1",
	})

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned", out.URL)
}

func (s *HandleGetKeyboardImageURLSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleGetKeyboardImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardImageURLInput{KeyboardID: " ", ImageID: "img-1"})

	s.Require().ErrorContains(err, "keyboard_id must not be blank")
}

func (s *HandleGetKeyboardImageURLSuite) TestBlankImageID_ReturnsError() {
	handler := handleGetKeyboardImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardImageURLInput{KeyboardID: "kb-1", ImageID: " "})

	s.Require().ErrorContains(err, "image_id must not be blank")
}

func (s *HandleGetKeyboardImageURLSuite) TestKeyboardNotFound_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(nil, repository.ErrNotFound)

	handler := handleGetKeyboardImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardImageURLInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

func (s *HandleGetKeyboardImageURLSuite) TestImageNotFound_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := handleGetKeyboardImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardImageURLInput{KeyboardID: "kb-1", ImageID: "missing"})

	s.Require().ErrorIs(err, errKeyboardImageNotFound)
}

func (s *HandleGetKeyboardImageURLSuite) TestPresignError_ReturnsError() {
	imagePath := repository.KeyboardImageKey("keyboards/caller-0001/kb-1/images/img-1")
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{
			ID:         "kb-1",
			Visibility: repository.VisibilityPrivate,
			Images:     []repository.KeyboardImage{{ImageID: "img-1", Path: imagePath}},
		}, nil)
	s.mockImages.EXPECT().
		PresignGetKeyboardImage(mock.Anything, imagePath).
		Return("", errors.New("s3: access denied"))

	handler := handleGetKeyboardImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardImageURLInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().ErrorContains(err, "failed to presign keyboard image")
}

type HandleAddKeyboardImageSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
	mockImages    *mocks.MockKeyboardImageStore
}

func TestHandleAddKeyboardImageSuite(t *testing.T) {
	suite.Run(t, new(HandleAddKeyboardImageSuite))
}

func (s *HandleAddKeyboardImageSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
}

func (s *HandleAddKeyboardImageSuite) TestSucceeds() {
	s.mockKeyboards.EXPECT().
		AddImage(mock.Anything, "kb-1", mock.MatchedBy(func(img repository.KeyboardImage) bool {
			return img.ImageID != ""
		})).
		Return(&repository.KeyboardImage{ImageID: "img-1"}, nil)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/png").
		Return("https://example.com/upload", nil)

	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "image/png",
	})

	s.Require().NoError(err)
	s.NotEmpty(out.ImageID)
	s.Equal("https://example.com/upload", out.UploadURL)
}

func (s *HandleAddKeyboardImageSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  " ",
		ContentType: "image/png",
	})

	s.Require().ErrorContains(err, "keyboard_id must not be blank")
}

func (s *HandleAddKeyboardImageSuite) TestUnapprovedContentType_ReturnsError() {
	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "application/pdf",
	})

	s.Require().ErrorContains(err, "content_type")
}

func (s *HandleAddKeyboardImageSuite) TestNotFound_ReturnsError() {
	s.mockKeyboards.EXPECT().
		AddImage(mock.Anything, "kb-1", mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "image/png",
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleAddKeyboardImageSuite) TestMutationConflict_ReturnsError() {
	s.mockKeyboards.EXPECT().
		AddImage(mock.Anything, "kb-1", mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "image/png",
	})

	s.Require().ErrorIs(err, errMutationConflict)
}

type HandleDeleteKeyboardImageSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
	mockImages    *mocks.MockKeyboardImageStore
}

func TestHandleDeleteKeyboardImageSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeyboardImageSuite))
}

func (s *HandleDeleteKeyboardImageSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
}

func (s *HandleDeleteKeyboardImageSuite) TestSucceeds() {
	key := repository.KeyboardImageKey("keyboards/u/kb-1/images/img-1")
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Images: []repository.KeyboardImage{
			{ImageID: "img-1", Path: key},
		}}, nil)
	s.mockImages.EXPECT().DeleteKeyboardImage(mock.Anything, key).Return(nil)
	s.mockKeyboards.EXPECT().DeleteImage(mock.Anything, "kb-1", "img-1").Return(&key, nil)

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeyboardImageSuite) TestBlankKeyboardID_ReturnsError() {
	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: " ", ImageID: "img-1"})

	s.Require().ErrorContains(err, "keyboard_id must not be blank")
}

func (s *HandleDeleteKeyboardImageSuite) TestBlankImageID_ReturnsError() {
	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: " "})

	s.Require().ErrorContains(err, "image_id must not be blank")
}

func (s *HandleDeleteKeyboardImageSuite) TestNotFound_ReturnsError() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteKeyboardImageSuite) TestMutationConflict_ReturnsError() {
	key := repository.KeyboardImageKey("keyboards/u/kb-1/images/img-1")
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Images: []repository.KeyboardImage{
			{ImageID: "img-1", Path: key},
		}}, nil)
	s.mockImages.EXPECT().DeleteKeyboardImage(mock.Anything, key).Return(nil)
	s.mockKeyboards.EXPECT().
		DeleteImage(mock.Anything, "kb-1", "img-1").
		Return(nil, repository.ErrMutationConflict)

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleDeleteKeyboardImageSuite) TestAlreadyAbsent_SucceedsWithoutS3Call() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeyboardImageSuite) TestS3DeleteFails_ReturnsError_DoesNotDeleteDBRecord() {
	key := repository.KeyboardImageKey("keyboards/u/kb-1/images/img-1")
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Images: []repository.KeyboardImage{
			{ImageID: "img-1", Path: key},
		}}, nil)
	s.mockImages.EXPECT().DeleteKeyboardImage(mock.Anything, key).Return(errors.New("s3 unavailable"))

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().Error(err)
	// mockKeyboards has no .EXPECT() for DeleteImage - verifies the DB
	// record was never touched.
}
