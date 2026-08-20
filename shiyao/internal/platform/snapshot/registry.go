package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Store defines the low-level object storage backend.
type Store interface {
	PutObject(ctx context.Context, key string, body io.Reader) error
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
}

// Registry manages storing and retrieving snapshot manifests and artifacts.
type Registry struct {
	store Store
}

func NewRegistry(store Store) (*Registry, error) {
	if store == nil {
		return nil, fmt.Errorf("storage driver is required")
	}
	return &Registry{store: store}, nil
}

// --- Local Disk Driver ---

type LocalStore struct {
	Root string
}

func NewLocalStore(root string) (*LocalStore, error) {
	if root == "" {
		return nil, fmt.Errorf("local store root path is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create local store directory: %w", err)
	}
	return &LocalStore{Root: root}, nil
}

func (l *LocalStore) PutObject(ctx context.Context, key string, body io.Reader) error {
	path := filepath.Join(l.Root, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create local directory: %w", err)
	}

	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open local destination file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, body); err != nil {
		return fmt.Errorf("write local file: %w", err)
	}
	return nil
}

func (l *LocalStore) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(l.Root, key)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open local file: %w", err)
	}
	return file, nil
}

// --- S3 / MinIO Driver ---

type S3Config struct {
	Endpoint        string // Set for MinIO or custom S3 (e.g., http://localhost:9000)
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool // Required for MinIO
}

type S3Store struct {
	client *s3.Client
	bucket string
}

func NewS3Store(ctx context.Context, cfg S3Config) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket name is required")
	}

	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &S3Store{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

func (s *S3Store) PutObject(ctx context.Context, key string, body io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("s3 put object %s: %w", key, err)
	}
	return nil
}

func (s *S3Store) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object %s: %w", key, err)
	}
	return out.Body, nil
}

// --- Registry Operations ---

func (r *Registry) Put(ctx context.Context, manifest Manifest) error {
	if manifest.Name == "" {
		return fmt.Errorf("snapshot manifest name is required")
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot manifest: %w", err)
	}

	key := fmt.Sprintf("%s/manifest.json", manifest.Name)
	return r.store.PutObject(ctx, key, bytes.NewReader(data))
}

func (r *Registry) Get(ctx context.Context, name string) (Manifest, error) {
	key := fmt.Sprintf("%s/manifest.json", name)
	reader, err := r.store.GetObject(ctx, key)
	if err != nil {
		return Manifest{}, fmt.Errorf("get manifest: %w", err)
	}
	defer reader.Close()

	var manifest Manifest
	if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode snapshot manifest: %w", err)
	}
	return manifest, nil
}

// PushArtifact streams a rootfs or kernel image from local disk into the registry.
func (r *Registry) PushArtifact(ctx context.Context, snapshotName, artifactName, localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open artifact file: %w", err)
	}
	defer file.Close()

	key := fmt.Sprintf("%s/%s", snapshotName, artifactName)
	return r.store.PutObject(ctx, key, file)
}

// PullArtifact streams an artifact from the registry onto local disk for VM execution.
func (r *Registry) PullArtifact(ctx context.Context, snapshotName, artifactName, targetPath string) error {
	key := fmt.Sprintf("%s/%s", snapshotName, artifactName)
	reader, err := r.store.GetObject(ctx, key)
	if err != nil {
		return fmt.Errorf("get artifact from store: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open local artifact destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("stream artifact to local disk: %w", err)
	}

	return nil
}
