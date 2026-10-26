package infrastructure

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"__GO_MODULE__/internal/modules/files/application"
)

var _ application.Storage = Disk{}

// Disk keeps the bytes under Root (DATA_DIR/files) as <yyyy>/<mm>/<random>. A write goes to a
// .part file first and is renamed when complete, so a half-written file is never served.
type Disk struct {
	Root string
}

var keyPattern = regexp.MustCompile(`^\d{4}/\d{2}/[0-9a-f]{32}$`)

// Check makes sure Root exists and can be written — the API refuses to start otherwise, rather
// than failing the first upload.
func (d Disk) Check() error {
	if err := os.MkdirAll(d.Root, 0o750); err != nil {
		return fmt.Errorf("files: cannot create %s: %w (on the server, data/ must belong to uid 10001 — deploy.sh sees to it)", d.Root, err)
	}
	probe := filepath.Join(d.Root, ".write-test")
	if err := os.WriteFile(probe, nil, 0o640); err != nil {
		return fmt.Errorf("files: %s is not writable: %w", d.Root, err)
	}
	return os.Remove(probe)
}

func (d Disk) Write(ctx context.Context, r io.Reader, limit int64) (string, int64, string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", 0, "", err
	}
	now := time.Now().UTC()
	key := fmt.Sprintf("%04d/%02d/%s", now.Year(), now.Month(), hex.EncodeToString(b))
	full := filepath.Join(d.Root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return "", 0, "", err
	}
	tmp := full + ".part"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", 0, "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(ctxReader{ctx, r}, limit+1))
	if err == nil && n > limit {
		err = application.ErrTooLarge
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, full)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", 0, "", err
	}
	return key, n, hex.EncodeToString(h.Sum(nil)), nil
}

func (d Disk) path(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", fmt.Errorf("files: malformed storage key %q", key)
	}
	return filepath.Join(d.Root, filepath.FromSlash(key)), nil
}

func (d Disk) Open(key string) (io.ReadSeekCloser, error) {
	p, err := d.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func (d Disk) Remove(key string) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (d Disk) CleanPartials(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return filepath.WalkDir(d.Root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(p, ".part") {
			return nil
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(p)
		}
		return nil
	})
}

// ctxReader stops a copy when the request ends.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
