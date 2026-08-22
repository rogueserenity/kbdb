package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type NewKeycapKitImageKeySuite struct {
	suite.Suite
}

func TestNewKeycapKitImageKeySuite(t *testing.T) {
	suite.Run(t, new(NewKeycapKitImageKeySuite))
}

func (s *NewKeycapKitImageKeySuite) TestSucceeds() {
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")

	key, err := repository.NewKeycapKitImageKey(ctx, "ks1", "kit1")

	s.Require().NoError(err)
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image"), key)
}

func (s *NewKeycapKitImageKeySuite) TestNoUserIDInContext_ReturnsError() {
	key, err := repository.NewKeycapKitImageKey(context.Background(), "ks1", "kit1")

	s.Require().ErrorIs(err, repository.ErrNoUserID)
	s.Empty(key)
}

type AggregateOrderStatusSuite struct {
	suite.Suite
}

func TestAggregateOrderStatusSuite(t *testing.T) {
	suite.Run(t, new(AggregateOrderStatusSuite))
}

func kitWithStatus(status string) repository.KeycapKit {
	s := status
	return repository.KeycapKit{Purchase: repository.KeycapKitPurchase{OrderStatus: &s}}
}

func (s *AggregateOrderStatusSuite) TestNoKits_ReturnsNil() {
	s.Nil(repository.AggregateOrderStatus(nil))
}

func (s *AggregateOrderStatusSuite) TestNoKitHasStatusSet_ReturnsNil() {
	kits := []repository.KeycapKit{{}, {}}

	s.Nil(repository.AggregateOrderStatus(kits))
}

func (s *AggregateOrderStatusSuite) TestAllKitsSameStatus_ReturnsThatStatus() {
	kits := []repository.KeycapKit{kitWithStatus("Shipped"), kitWithStatus("Shipped")}

	got := repository.AggregateOrderStatus(kits)

	s.Require().NotNil(got)
	s.Equal("Shipped", *got)
}

func (s *AggregateOrderStatusSuite) TestMixedStatuses_ReturnsLeastProgressed() {
	kits := []repository.KeycapKit{kitWithStatus("Delivered"), kitWithStatus("Ordered")}

	got := repository.AggregateOrderStatus(kits)

	s.Require().NotNil(got)
	s.Equal("Ordered", *got)
}

func (s *AggregateOrderStatusSuite) TestCancelledKitAmongOthers_IsExcludedFromComparison() {
	kits := []repository.KeycapKit{kitWithStatus("Delivered"), kitWithStatus("Cancelled")}

	got := repository.AggregateOrderStatus(kits)

	s.Require().NotNil(got)
	s.Equal("Delivered", *got)
}

func (s *AggregateOrderStatusSuite) TestAllKitsCancelled_ReturnsCancelled() {
	kits := []repository.KeycapKit{kitWithStatus("Cancelled"), kitWithStatus("Cancelled")}

	got := repository.AggregateOrderStatus(kits)

	s.Require().NotNil(got)
	s.Equal("Cancelled", *got)
}

func (s *AggregateOrderStatusSuite) TestUnrecognizedStatus_ReturnsNil() {
	kits := []repository.KeycapKit{kitWithStatus("SomethingUnexpected")}

	s.Nil(repository.AggregateOrderStatus(kits))
}
