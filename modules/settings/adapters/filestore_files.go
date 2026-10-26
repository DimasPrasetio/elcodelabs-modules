package modules

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	filescontracts "__GO_MODULE__/internal/modules/files/contracts"
	settingsports "__GO_MODULE__/internal/modules/settings/ports"
)

// settingsFileStore satisfies settings' FileStore port with the files module's client.
// Mapping only.
type settingsFileStore struct {
	files filescontracts.FileClient
}

func (a settingsFileStore) Save(ctx context.Context, name string, r io.Reader, maxBytes int64, allowed []string) (int64, error) {
	f, err := a.files.Save(ctx, filescontracts.Upload{Owner: "settings", Name: name, Reader: r, MaxBytes: maxBytes, Allowed: allowed})
	if errors.Is(err, filescontracts.ErrRejected) {
		return 0, fmt.Errorf("%w: %s", settingsports.ErrFileRejected, strings.TrimPrefix(err.Error(), filescontracts.ErrRejected.Error()+": "))
	}
	return f.ID, err
}

func (a settingsFileStore) Open(ctx context.Context, id int64) (io.ReadCloser, string, error) {
	f, rc, err := a.files.Open(ctx, id)
	if errors.Is(err, filescontracts.ErrNotFound) {
		return nil, "", settingsports.ErrFileNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return rc, f.ContentType, nil
}

func (a settingsFileStore) Delete(ctx context.Context, id int64) error {
	err := a.files.Delete(ctx, id)
	if errors.Is(err, filescontracts.ErrNotFound) {
		return nil
	}
	return err
}
