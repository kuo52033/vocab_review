package audios

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Presigner struct {
	client  *s3.PresignClient
	bucket  string
	expires time.Duration
}

func NewS3Presigner(config aws.Config, bucket string, expires time.Duration) *S3Presigner {
	if expires <= 0 {
		expires = 5 * time.Minute
	}
	return &S3Presigner{
		client:  s3.NewPresignClient(s3.NewFromConfig(config)),
		bucket:  bucket,
		expires: expires,
	}
}

func (p *S3Presigner) SignVocabAudioURL(ctx context.Context, storageKey string) (string, error) {
	result, err := p.client.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(storageKey),
	}, func(options *s3.PresignOptions) {
		options.Expires = p.expires
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
