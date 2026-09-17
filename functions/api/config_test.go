package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) validConfig() Config {
	return Config{
		ImageGetPresignBucket: 24 * time.Hour,
		ImageGetPresignMinTTL: 3 * time.Hour,
	}
}

func (s *ConfigSuite) TestValidate_DefaultBucketAndMinTTL_Succeeds() {
	s.Require().NoError(s.validConfig().Validate())
}

func (s *ConfigSuite) TestValidate_MinTTLEqualsBucket_Errors() {
	cfg := s.validConfig()
	cfg.ImageGetPresignMinTTL = cfg.ImageGetPresignBucket

	s.Require().ErrorContains(cfg.Validate(), "must be less than")
}

func (s *ConfigSuite) TestValidate_MinTTLGreaterThanBucket_Errors() {
	cfg := s.validConfig()
	cfg.ImageGetPresignMinTTL = cfg.ImageGetPresignBucket + time.Hour

	s.Require().ErrorContains(cfg.Validate(), "must be less than")
}

func (s *ConfigSuite) TestValidate_BucketDoesNotDivide24h_Errors() {
	cfg := s.validConfig()
	cfg.ImageGetPresignBucket = 5 * time.Hour
	cfg.ImageGetPresignMinTTL = time.Hour

	s.Require().ErrorContains(cfg.Validate(), "must evenly divide 24h")
}

func (s *ConfigSuite) TestValidate_ZeroBucket_Errors() {
	cfg := s.validConfig()
	cfg.ImageGetPresignBucket = 0
	cfg.ImageGetPresignMinTTL = -time.Hour

	s.Require().ErrorContains(cfg.Validate(), "must evenly divide 24h")
}

func (s *ConfigSuite) TestValidate_BucketEvenlyDividing24h_Succeeds() {
	cfg := s.validConfig()
	cfg.ImageGetPresignBucket = 6 * time.Hour
	cfg.ImageGetPresignMinTTL = time.Hour

	s.Require().NoError(cfg.Validate())
}
