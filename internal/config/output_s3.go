package config

import (
	"fmt"
	"time"
)

const (
	// DefaultS3Region is the default AWS region for S3 output
	DefaultS3Region = "us-east-1"
	// DefaultS3Workers is the default number of workers for S3 output
	DefaultS3Workers = 1
	// DefaultS3BatchTimeout is the default batch timeout for S3 output
	DefaultS3BatchTimeout = 30 * time.Second
	// DefaultS3BatchSize is the default number of logs per S3 batch
	DefaultS3BatchSize = 10000
)

// S3OutputConfig contains configuration for S3 output
type S3OutputConfig struct {
	// Bucket is the target S3 bucket name (required for S3 output)
	Bucket string `yaml:"bucket,omitempty" mapstructure:"bucket,omitempty"`
	// Region is the AWS region
	Region string `yaml:"region,omitempty" mapstructure:"region,omitempty"`
	// KeyPrefix is an optional key prefix for uploaded objects
	KeyPrefix string `yaml:"keyPrefix,omitempty" mapstructure:"keyPrefix,omitempty"`
	// Workers is the number of worker goroutines for S3 output
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// BatchTimeout is the timeout for batching log records
	BatchTimeout time.Duration `yaml:"batchTimeout,omitempty" mapstructure:"batchTimeout,omitempty"`
	// BatchSize is the number of logs per batch
	BatchSize int `yaml:"batchSize,omitempty" mapstructure:"batchSize,omitempty"`

	// Optional static credentials. When unset, the AWS SDK default credential chain is used.
	AccessKeyID     string `yaml:"accessKeyID,omitempty" mapstructure:"accessKeyID,omitempty"`
	SecretAccessKey string `yaml:"secretAccessKey,omitempty" mapstructure:"secretAccessKey,omitempty"`
}

// Validate validates the S3 output configuration
func (c *S3OutputConfig) Validate() error {
	if c.Workers < 0 {
		return fmt.Errorf("S3 output workers cannot be negative, got %d", c.Workers)
	}
	if c.BatchSize < 0 {
		return fmt.Errorf("S3 output batch size cannot be negative, got %d", c.BatchSize)
	}
	if c.BatchTimeout < 0 {
		return fmt.Errorf("S3 output batch timeout cannot be negative, got %s", c.BatchTimeout)
	}
	// Bucket is required only when type=s3 is selected; presence is validated in main when instantiating
	return nil
}
