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

func validKeycapSetInput() schema.KeycapSetInput {
	return schema.KeycapSetInput{
		Brand:      "GMK",
		Name:       "Olivia",
		Visibility: "private",
	}
}

type HandleCreateKeycapSetSuite struct {
	suite.Suite

	mockKeycapSets *mocks.MockKeycapSetRepository
}

func TestHandleCreateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateKeycapSetSuite))
}

func (s *HandleCreateKeycapSetSuite) SetupTest() {
	s.mockKeycapSets = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleCreateKeycapSetSuite) TestSucceeds() {
	s.mockKeycapSets.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, ks repository.KeycapSet) (*repository.KeycapSet, error) {
			return &ks, nil
		})

	handler := handleCreateKeycapSet(s.mockKeycapSets)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: validKeycapSetInput()})

	s.Require().NoError(err)
	s.Equal("GMK", out.KeycapSet.Brand)
	s.NotEmpty(out.KeycapSet.ID, "create must assign a server-generated id")
}

func (s *HandleCreateKeycapSetSuite) TestBlankBrand_ReturnsError() {
	in := validKeycapSetInput()
	in.Brand = "   "

	handler := handleCreateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: in})

	s.Require().ErrorContains(err, "brand must not be blank")
}

func (s *HandleCreateKeycapSetSuite) TestBlankName_ReturnsError() {
	in := validKeycapSetInput()
	in.Name = "  "

	handler := handleCreateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: in})

	s.Require().ErrorContains(err, "name must not be blank")
}

func (s *HandleCreateKeycapSetSuite) TestInvalidVisibility_ReturnsError() {
	in := validKeycapSetInput()
	in.Visibility = "everyone"

	handler := handleCreateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: in})

	s.Require().ErrorContains(err, "visibility")
}

func (s *HandleCreateKeycapSetSuite) TestUnapprovedProfile_ReturnsError() {
	profile := "NotAProfile"
	in := validKeycapSetInput()
	in.Profile = &profile

	handler := handleCreateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: in})

	s.Require().ErrorContains(err, "profile")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateKeycapSetSuite) TestUnapprovedMaterial_ReturnsError() {
	material := "NotAMaterial"
	in := validKeycapSetInput()
	in.Material = &material

	handler := handleCreateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: in})

	s.Require().ErrorContains(err, "material")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateKeycapSetSuite) TestAlreadyExists_ReturnsAlreadyExists() {
	s.mockKeycapSets.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	handler := handleCreateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: validKeycapSetInput()})

	s.Require().ErrorIs(err, errKeycapSetAlreadyExists)
}

func (s *HandleCreateKeycapSetSuite) TestRepositoryError_ReturnsError() {
	s.mockKeycapSets.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("create failed"))

	handler := handleCreateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: validKeycapSetInput()})

	s.Require().ErrorContains(err, "failed to create keycap set")
}

type HandleUpdateKeycapSetSuite struct {
	suite.Suite

	mockKeycapSets *mocks.MockKeycapSetRepository
}

func TestHandleUpdateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateKeycapSetSuite))
}

func (s *HandleUpdateKeycapSetSuite) SetupTest() {
	s.mockKeycapSets = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleUpdateKeycapSetSuite) TestSucceeds() {
	s.mockKeycapSets.EXPECT().
		Update(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, ks repository.KeycapSet) (*repository.KeycapSet, error) {
			return &ks, nil
		})

	handler := handleUpdateKeycapSet(s.mockKeycapSets)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{
		KeycapSetID:    "ks-1",
		KeycapSetInput: validKeycapSetInput(),
	})

	s.Require().NoError(err)
	s.Equal("ks-1", out.KeycapSet.ID, "update must target the requested id")
}

func (s *HandleUpdateKeycapSetSuite) TestBlankKeycapSetID_ReturnsError() {
	handler := handleUpdateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{
		KeycapSetID:    "  ",
		KeycapSetInput: validKeycapSetInput(),
	})

	s.Require().ErrorContains(err, "keycap_set_id must not be blank")
}

func (s *HandleUpdateKeycapSetSuite) TestNotFound_ReturnsNotFound() {
	s.mockKeycapSets.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleUpdateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{
		KeycapSetID:    "missing",
		KeycapSetInput: validKeycapSetInput(),
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleUpdateKeycapSetSuite) TestMutationConflict_ReturnsConflictError() {
	s.mockKeycapSets.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	handler := handleUpdateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{
		KeycapSetID:    "ks-1",
		KeycapSetInput: validKeycapSetInput(),
	})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleUpdateKeycapSetSuite) TestRepositoryError_ReturnsError() {
	s.mockKeycapSets.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("update failed"))

	handler := handleUpdateKeycapSet(s.mockKeycapSets)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{
		KeycapSetID:    "ks-1",
		KeycapSetInput: validKeycapSetInput(),
	})

	s.Require().ErrorIs(err, errMutationFailed)
}

type HandleDeleteKeycapSetSuite struct {
	suite.Suite

	mockRepo     *mocks.MockKeycapSetRepository
	mockBuilds   *mocks.MockBuildRepository
	mockBuildImg *mocks.MockBuildImageStore
	mockImages   *mocks.MockKeycapKitImageStore
}

func TestHandleDeleteKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeycapSetSuite))
}

func (s *HandleDeleteKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImg = mocks.NewMockBuildImageStore(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
}

func (s *HandleDeleteKeycapSetSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "ks-1").Return(nil)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeycapSetSuite) TestBlankKeycapSetID_ReturnsError() {
	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: ""})

	s.Require().ErrorContains(err, "keycap_set_id must not be blank")
}

func (s *HandleDeleteKeycapSetSuite) TestInvalidOnDelete_ReturnsError() {
	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1", OnDelete: "bogus"})

	s.Require().Error(err)
}

func (s *HandleDeleteKeycapSetSuite) TestNotFound_StillSucceeds() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "missing").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "missing"})

	s.Require().NoError(err, "delete is idempotent: a nonexistent id is not an error")
}

func (s *HandleDeleteKeycapSetSuite) TestKitImagesAreDeletedFromS3BeforeDB() {
	key1 := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image")
	key2 := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-2/image")
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1", ImagePath: &key1},
			{KitID: "kit-2", ImagePath: &key2},
		}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key1).Return(nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key2).Return(nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "ks-1").Return(nil)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeycapSetSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, mock.Anything).Return(errors.New("delete failed"))

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().ErrorContains(err, "failed to delete keycap set")
}

func (s *HandleDeleteKeycapSetSuite) TestKitImageDeleteFails_ReturnsError_DoesNotDeleteSet() {
	key1 := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image")
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1", ImagePath: &key1},
		}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key1).Return(errors.New("s3 unavailable"))

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().Error(err)
	// mockRepo has no .EXPECT() for Delete - verifies the DB record was
	// never touched.
}

func (s *HandleDeleteKeycapSetSuite) TestBlock_Referenced_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").
		Return([]string{"build-1"}, nil)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1", OnDelete: "block"})

	s.Require().ErrorContains(err, "build-1")
}

func (s *HandleDeleteKeycapSetSuite) TestDetach_Referenced_Succeeds_DoesNotCheckReferences() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "ks-1").Return(nil)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1", OnDelete: "detach"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeycapSetSuite) TestCascade_Referenced_ReturnsDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "ks-1").Return(nil)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"build-1"}, out.DeletedBuildIDs)
}

func validKeycapKitInput() schema.KeycapKitInput {
	return schema.KeycapKitInput{Name: "Base"}
}

type HandleCreateKeycapKitSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeycapSetRepository
}

func TestHandleCreateKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateKeycapKitSuite))
}

func (s *HandleCreateKeycapKitSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleCreateKeycapKitSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	handler := handleCreateKeycapKit(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().NoError(err)
	s.Equal("Base", out.KeycapKit.Name)
	s.NotEmpty(out.KeycapKit.KitID, "create must assign a server-generated id")
}

func (s *HandleCreateKeycapKitSuite) TestPrimaryTrue_PassesPrimaryToRepo() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks-1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary != nil && *primary
		})).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	in := validKeycapKitInput()
	primary := true
	in.Primary = &primary

	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: in,
	})

	s.Require().NoError(err)
}

func (s *HandleCreateKeycapKitSuite) TestPrimaryOmitted_PassesNilToRepo() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks-1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary == nil
		})).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().NoError(err)
}

func (s *HandleCreateKeycapKitSuite) TestBlankKeycapSetID_ReturnsError() {
	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "  ",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorContains(err, "keycap_set_id must not be blank")
}

func (s *HandleCreateKeycapKitSuite) TestBlankName_ReturnsError() {
	in := validKeycapKitInput()
	in.Name = " "

	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: in,
	})

	s.Require().ErrorContains(err, "name must not be blank")
}

func (s *HandleCreateKeycapKitSuite) TestMalformedOrderDate_ReturnsError() {
	bad := "not-a-date"
	in := validKeycapKitInput()
	in.Purchase = &schema.KeycapKitPurchase{OrderDate: &bad}

	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: in,
	})

	s.Require().ErrorContains(err, "purchase.order_date")
}

func (s *HandleCreateKeycapKitSuite) TestUnapprovedVendor_ReturnsError() {
	vendor := "NotARealVendor"
	in := validKeycapKitInput()
	in.Purchase = &schema.KeycapKitPurchase{Vendor: &vendor}

	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: in,
	})

	s.Require().ErrorContains(err, "purchase.vendor")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateKeycapKitSuite) TestUnapprovedOrderStatus_ReturnsError() {
	status := "Bogus"
	in := validKeycapKitInput()
	in.Purchase = &schema.KeycapKitPurchase{OrderStatus: &status}

	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: in,
	})

	s.Require().ErrorContains(err, "purchase.order_status")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateKeycapKitSuite) TestKeycapSetNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "missing", mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "missing",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleCreateKeycapKitSuite) TestMutationConflict_ReturnsConflictError() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleCreateKeycapKitSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(nil, errors.New("add kit failed"))

	handler := handleCreateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationFailed)
}

type HandleUpdateKeycapKitSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeycapSetRepository
}

func TestHandleUpdateKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateKeycapKitSuite))
}

func (s *HandleUpdateKeycapKitSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleUpdateKeycapKitSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	handler := handleUpdateKeycapKit(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().NoError(err)
	s.Equal("kit-1", out.KeycapKit.KitID, "update must target the requested id")
}

func (s *HandleUpdateKeycapKitSuite) TestPrimaryFalse_PassesPrimaryToRepo() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary != nil && !*primary
		})).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	in := validKeycapKitInput()
	primary := false
	in.Primary = &primary

	handler := handleUpdateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: in,
	})

	s.Require().NoError(err)
}

func (s *HandleUpdateKeycapKitSuite) TestPrimaryOmitted_PassesNilToRepo() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary == nil
		})).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	handler := handleUpdateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().NoError(err)
}

func (s *HandleUpdateKeycapKitSuite) TestBlankKeycapSetID_ReturnsError() {
	handler := handleUpdateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    " ",
		KitID:          "kit-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorContains(err, "keycap_set_id must not be blank")
}

func (s *HandleUpdateKeycapKitSuite) TestBlankKitID_ReturnsError() {
	handler := handleUpdateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          " ",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorContains(err, "kit_id must not be blank")
}

func (s *HandleUpdateKeycapKitSuite) TestUnapprovedVendor_ReturnsError() {
	vendor := "NotARealVendor"
	in := validKeycapKitInput()
	in.Purchase = &schema.KeycapKitPurchase{Vendor: &vendor}

	handler := handleUpdateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: in,
	})

	s.Require().ErrorContains(err, "purchase.vendor")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleUpdateKeycapKitSuite) TestKitNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleUpdateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "missing-kit",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleUpdateKeycapKitSuite) TestMutationConflict_ReturnsConflictError() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	handler := handleUpdateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleUpdateKeycapKitSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(nil, errors.New("update kit failed"))

	handler := handleUpdateKeycapKit(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationFailed)
}

type HandleDeleteKeycapKitSuite struct {
	suite.Suite

	mockRepo     *mocks.MockKeycapSetRepository
	mockBuilds   *mocks.MockBuildRepository
	mockBuildImg *mocks.MockBuildImageStore
	mockImages   *mocks.MockKeycapKitImageStore
}

func TestHandleDeleteKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeycapKitSuite))
}

func (s *HandleDeleteKeycapKitSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImg = mocks.NewMockBuildImageStore(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
}

func (s *HandleDeleteKeycapKitSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1"},
		}}, nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeycapKitSuite) TestBlankKeycapSetID_ReturnsError() {
	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: " ", KitID: "kit-1"})

	s.Require().ErrorContains(err, "keycap_set_id must not be blank")
}

func (s *HandleDeleteKeycapKitSuite) TestBlankKitID_ReturnsError() {
	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: " "})

	s.Require().ErrorContains(err, "kit_id must not be blank")
}

func (s *HandleDeleteKeycapKitSuite) TestInvalidOnDelete_ReturnsError() {
	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1", OnDelete: "bogus"})

	s.Require().Error(err)
}

func (s *HandleDeleteKeycapKitSuite) TestKitIDNotInSet_StillSucceeds() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "missing-kit").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "missing-kit"})

	s.Require().NoError(err, "delete is idempotent: a kit id not present in the set is not an error")
}

func (s *HandleDeleteKeycapKitSuite) TestKeycapSetNotFound_ReturnsNotFound() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "missing", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "missing", KitID: "kit-1"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteKeycapKitSuite) TestMutationConflict_ReturnsConflictError() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1"},
		}}, nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(repository.ErrMutationConflict)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleDeleteKeycapKitSuite) TestImageIsDeletedFromS3BeforeDB() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image")
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1", ImagePath: &key},
		}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeycapKitSuite) TestImageDeleteFails_ReturnsError_DoesNotDeleteKit() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image")
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1", ImagePath: &key},
		}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(errors.New("s3 delete failed"))

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().Error(err)
	// mockRepo has no .EXPECT() for DeleteKit - verifies the DB record was
	// never touched.
}

func (s *HandleDeleteKeycapKitSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1"},
		}}, nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(errors.New("delete kit failed"))

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().ErrorIs(err, errMutationFailed)
}

func (s *HandleDeleteKeycapKitSuite) TestBlock_Referenced_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").
		Return([]string{"build-1"}, nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1", OnDelete: "block"})

	s.Require().ErrorContains(err, "build-1")
}

func (s *HandleDeleteKeycapKitSuite) TestDetach_Referenced_Succeeds_DoesNotCheckReferences() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1"},
		}}, nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1", OnDelete: "detach"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeycapKitSuite) TestCascade_Referenced_ReturnsDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1"},
		}}, nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"build-1"}, out.DeletedBuildIDs)
}

type HandleSetKeycapKitImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
}

func TestHandleSetKeycapKitImageSuite(t *testing.T) {
	suite.Run(t, new(HandleSetKeycapKitImageSuite))
}

func (s *HandleSetKeycapKitImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
}

func (s *HandleSetKeycapKitImageSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks-1", "kit-1", mock.Anything).
		Return(&repository.KeycapKit{KitID: "kit-1"}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, mock.Anything, "image/png").
		Return("https://example.com/upload", nil)

	handler := handleSetKeycapKitImage(s.mockRepo, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.SetKeycapKitImageInput{
		KeycapSetID: "ks-1",
		KitID:       "kit-1",
		ContentType: "image/png",
	})

	s.Require().NoError(err)
	s.Equal("https://example.com/upload", out.UploadURL)
}

func (s *HandleSetKeycapKitImageSuite) TestBlankKeycapSetID_ReturnsError() {
	handler := handleSetKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.SetKeycapKitImageInput{
		KeycapSetID: " ",
		KitID:       "kit-1",
		ContentType: "image/png",
	})

	s.Require().ErrorContains(err, "keycap_set_id must not be blank")
}

func (s *HandleSetKeycapKitImageSuite) TestBlankKitID_ReturnsError() {
	handler := handleSetKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.SetKeycapKitImageInput{
		KeycapSetID: "ks-1",
		KitID:       " ",
		ContentType: "image/png",
	})

	s.Require().ErrorContains(err, "kit_id must not be blank")
}

func (s *HandleSetKeycapKitImageSuite) TestUnapprovedContentType_ReturnsError() {
	handler := handleSetKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.SetKeycapKitImageInput{
		KeycapSetID: "ks-1",
		KitID:       "kit-1",
		ContentType: "application/exe",
	})

	s.Require().ErrorContains(err, "content_type")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleSetKeycapKitImageSuite) TestKitNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks-1", "missing-kit", mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleSetKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.SetKeycapKitImageInput{
		KeycapSetID: "ks-1",
		KitID:       "missing-kit",
		ContentType: "image/png",
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleSetKeycapKitImageSuite) TestMutationConflict_ReturnsConflictError() {
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks-1", "kit-1", mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	handler := handleSetKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.SetKeycapKitImageInput{
		KeycapSetID: "ks-1",
		KitID:       "kit-1",
		ContentType: "image/png",
	})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleSetKeycapKitImageSuite) TestPresignError_ReturnsError() {
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks-1", "kit-1", mock.Anything).
		Return(&repository.KeycapKit{KitID: "kit-1"}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, mock.Anything, "image/png").
		Return("", errors.New("presign failed"))

	handler := handleSetKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.SetKeycapKitImageInput{
		KeycapSetID: "ks-1",
		KitID:       "kit-1",
		ContentType: "image/png",
	})

	s.Require().ErrorContains(err, "failed to set kit image")
}

type HandleDeleteKeycapKitImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
}

func TestHandleDeleteKeycapKitImageSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeycapKitImageSuite))
}

func (s *HandleDeleteKeycapKitImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
}

func (s *HandleDeleteKeycapKitImageSuite) TestSucceeds() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image")
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1", ImagePath: &key},
		}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil)
	s.mockRepo.EXPECT().ClearKitImagePath(mock.Anything, "ks-1", "kit-1").Return(&key, nil)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeycapKitImageSuite) TestBlankKeycapSetID_ReturnsError() {
	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: " ", KitID: "kit-1"})

	s.Require().ErrorContains(err, "keycap_set_id must not be blank")
}

func (s *HandleDeleteKeycapKitImageSuite) TestBlankKitID_ReturnsError() {
	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: " "})

	s.Require().ErrorContains(err, "kit_id must not be blank")
}

func (s *HandleDeleteKeycapKitImageSuite) TestAlreadyCleared_StillSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1"},
		}}, nil)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().NoError(err, "clearing an already-unset image is not an error")
}

func (s *HandleDeleteKeycapKitImageSuite) TestKitNotInSet_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "missing-kit"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteKeycapKitImageSuite) TestKitNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "missing-kit"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteKeycapKitImageSuite) TestMutationConflict_ReturnsConflictError() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image")
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1", ImagePath: &key},
		}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil)
	s.mockRepo.EXPECT().ClearKitImagePath(mock.Anything, "ks-1", "kit-1").Return(nil, repository.ErrMutationConflict)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().ErrorContains(err, "failed to delete kit image")
}

func (s *HandleDeleteKeycapKitImageSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(nil, errors.New("get failed"))

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().ErrorIs(err, errMutationFailed)
}

func (s *HandleDeleteKeycapKitImageSuite) TestImageDeleteFailure_ReturnsError_DoesNotClearDBRecord() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image")
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{
			{KitID: "kit-1", ImagePath: &key},
		}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(errors.New("s3 delete failed"))

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().ErrorContains(err, "failed to delete kit image")
	// mockRepo has no .EXPECT() for ClearKitImagePath - verifies the DB
	// record was never touched.
}
