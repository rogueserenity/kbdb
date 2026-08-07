package mcp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type HandleListKeycapSetsSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeycapSetRepository
}

func TestHandleListKeycapSetsSuite(t *testing.T) {
	suite.Run(t, new(HandleListKeycapSetsSuite))
}

func (s *HandleListKeycapSetsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleListKeycapSetsSuite) TestBlankUserID_DefaultsToCaller() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, defaultListLimit, "").
		Return([]repository.KeycapSet{{ID: "ks-1", Brand: "GMK", Name: "Olivia"}}, "", nil)

	handler := handleListKeycapSets(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSets, 1)
	s.Equal("ks-1", out.KeycapSets[0].ID)
}

func (s *HandleListKeycapSetsSuite) TestOwnCollection_ReadsAllVisibilityTiers() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
			repository.VisibilityPrivate,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)

	handler := handleListKeycapSets(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{})

	s.Require().NoError(err)
}

func (s *HandleListKeycapSetsSuite) TestOtherUsersCollection_ExcludesPrivate() {
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)

	handler := handleListKeycapSets(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{UserID: otherID})

	s.Require().NoError(err)
}

func (s *HandleListKeycapSetsSuite) TestLimitAboveMax_IsClamped() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, maxListLimit, mock.Anything).
		Return(nil, "", nil)

	handler := handleListKeycapSets(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{Limit: 5000})

	s.Require().NoError(err)
}

func (s *HandleListKeycapSetsSuite) TestCursorIsPropagated() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "page-2").
		Return(nil, "page-3", nil)

	handler := handleListKeycapSets(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{Cursor: "page-2"})

	s.Require().NoError(err)
	s.Equal("page-3", out.NextCursor)
}

func (s *HandleListKeycapSetsSuite) TestNoCallerIdentity_ReturnsError() {
	handler := handleListKeycapSets(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.ListKeycapSetsInput{})

	s.Require().ErrorIs(err, errNoCallerIdentity)
}

func (s *HandleListKeycapSetsSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", errors.New("query failed"))

	handler := handleListKeycapSets(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{})

	s.Require().ErrorContains(err, "failed to list keycap sets")
}

type HandleGetKeycapSetSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeycapSetRepository
}

func TestHandleGetKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(HandleGetKeycapSetSuite))
}

func (s *HandleGetKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleGetKeycapSetSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(&repository.KeycapSet{
			ID:         "ks-1",
			Brand:      "GMK",
			Name:       "Olivia",
			Visibility: repository.VisibilityPrivate,
		}, nil)

	handler := handleGetKeycapSet(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().NoError(err)
	s.Equal("ks-1", out.KeycapSet.ID)
	s.Equal("GMK", out.KeycapSet.Brand)
}

func (s *HandleGetKeycapSetSuite) TestKitsMapWithHasImage() {
	imagePath := repository.KeycapKitImageKey("keycap-sets/caller-0001/ks-1/kits/kit-1/image")
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(&repository.KeycapSet{
			ID:         "ks-1",
			Visibility: repository.VisibilityPrivate,
			Kits: []repository.KeycapKit{
				{KitID: "kit-1", Name: "Base", ImagePath: &imagePath},
				{KitID: "kit-2", Name: "Novelties"},
			},
		}, nil)

	handler := handleGetKeycapSet(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSet.Kits, 2)
	s.True(out.KeycapSet.Kits[0].HasImage)
	s.False(out.KeycapSet.Kits[1].HasImage)
}

func (s *HandleGetKeycapSetSuite) TestBlankKeycapSetID_ReturnsError() {
	handler := handleGetKeycapSet(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "  "})

	s.Require().ErrorContains(err, "keycap_set_id must not be blank")
}

func (s *HandleGetKeycapSetSuite) TestNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetKeycapSet(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "missing"})

	s.Require().ErrorIs(err, errKeycapSetNotFound)
}

func (s *HandleGetKeycapSetSuite) TestOtherUsersPrivateKeycapSet_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := handleGetKeycapSet(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1", UserID: otherID})

	s.Require().ErrorIs(err, errKeycapSetNotFound)
}

func (s *HandleGetKeycapSetSuite) TestOtherUsersPublicKeycapSet_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "ks-1").
		Return(&repository.KeycapSet{
			ID:         "ks-1",
			Brand:      "GMK",
			Visibility: repository.VisibilityPublic,
		}, nil)

	handler := handleGetKeycapSet(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1", UserID: otherID})

	s.Require().NoError(err)
	s.Equal("ks-1", out.KeycapSet.ID)
}

func (s *HandleGetKeycapSetSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("get failed"))

	handler := handleGetKeycapSet(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().ErrorContains(err, "failed to get keycap set")
}

type HandleGetKeycapKitImageURLSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
}

func TestHandleGetKeycapKitImageURLSuite(t *testing.T) {
	suite.Run(t, new(HandleGetKeycapKitImageURLSuite))
}

func (s *HandleGetKeycapKitImageURLSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
}

func (s *HandleGetKeycapKitImageURLSuite) TestSucceeds() {
	imagePath := repository.KeycapKitImageKey("keycap-sets/caller-0001/ks-1/kits/kit-1/image")
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(&repository.KeycapSet{
			ID:         "ks-1",
			Visibility: repository.VisibilityPrivate,
			Kits:       []repository.KeycapKit{{KitID: "kit-1", ImagePath: &imagePath}},
		}, nil)
	s.mockImages.EXPECT().
		PresignGet(mock.Anything, imagePath).
		Return("https://example.com/presigned", nil)

	handler := handleGetKeycapKitImageURL(s.mockRepo, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapKitImageURLInput{
		KeycapSetID: "ks-1",
		KitID:       "kit-1",
	})

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned", out.URL)
}

func (s *HandleGetKeycapKitImageURLSuite) TestBlankKeycapSetID_ReturnsError() {
	handler := handleGetKeycapKitImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapKitImageURLInput{
		KeycapSetID: "  ",
		KitID:       "kit-1",
	})

	s.Require().ErrorContains(err, "keycap_set_id must not be blank")
}

func (s *HandleGetKeycapKitImageURLSuite) TestBlankKitID_ReturnsError() {
	handler := handleGetKeycapKitImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapKitImageURLInput{
		KeycapSetID: "ks-1",
		KitID:       " ",
	})

	s.Require().ErrorContains(err, "kit_id must not be blank")
}

func (s *HandleGetKeycapKitImageURLSuite) TestKeycapSetNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetKeycapKitImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapKitImageURLInput{
		KeycapSetID: "missing",
		KitID:       "kit-1",
	})

	s.Require().ErrorIs(err, errKeycapSetNotFound)
}

func (s *HandleGetKeycapKitImageURLSuite) TestOtherUsersPrivateKeycapSet_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := handleGetKeycapKitImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapKitImageURLInput{
		KeycapSetID: "ks-1",
		KitID:       "kit-1",
		UserID:      otherID,
	})

	s.Require().ErrorIs(err, errKeycapSetNotFound)
}

func (s *HandleGetKeycapKitImageURLSuite) TestKitNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := handleGetKeycapKitImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapKitImageURLInput{
		KeycapSetID: "ks-1",
		KitID:       "missing-kit",
	})

	s.Require().ErrorIs(err, errKeycapKitNotFound)
}

func (s *HandleGetKeycapKitImageURLSuite) TestKitHasNoImage_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(&repository.KeycapSet{
			ID:         "ks-1",
			Visibility: repository.VisibilityPrivate,
			Kits:       []repository.KeycapKit{{KitID: "kit-1"}},
		}, nil)

	handler := handleGetKeycapKitImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapKitImageURLInput{
		KeycapSetID: "ks-1",
		KitID:       "kit-1",
	})

	s.Require().ErrorIs(err, errKeycapKitHasNoImage)
}

func (s *HandleGetKeycapKitImageURLSuite) TestPresignError_ReturnsError() {
	imagePath := repository.KeycapKitImageKey("keycap-sets/caller-0001/ks-1/kits/kit-1/image")
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(&repository.KeycapSet{
			ID:         "ks-1",
			Visibility: repository.VisibilityPrivate,
			Kits:       []repository.KeycapKit{{KitID: "kit-1", ImagePath: &imagePath}},
		}, nil)
	s.mockImages.EXPECT().
		PresignGet(mock.Anything, imagePath).
		Return("", errors.New("presign failed"))

	handler := handleGetKeycapKitImageURL(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapKitImageURLInput{
		KeycapSetID: "ks-1",
		KitID:       "kit-1",
	})

	s.Require().ErrorContains(err, "failed to presign keycap kit image")
}
