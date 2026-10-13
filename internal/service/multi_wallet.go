package service

import (
	"context"
	"errors"

	"native-ios-passkit-wip/internal/domain"
)

type CompositeWalletPassUpdater struct {
	updaters []WalletPassUpdater
}

func NewCompositeWalletPassUpdater(updaters ...WalletPassUpdater) CompositeWalletPassUpdater {
	return CompositeWalletPassUpdater{updaters: updaters}
}

func (u CompositeWalletPassUpdater) MarkPassUpdated(ctx context.Context, serialNumber string) error {
	for _, updater := range u.updaters {
		if updater == nil {
			continue
		}
		if err := updater.MarkPassUpdated(ctx, serialNumber); err != nil && !errors.Is(err, domain.ErrWalletPassNotFound) && !errors.Is(err, domain.ErrGoogleWalletObjectNotFound) {
			return err
		}
	}
	return nil
}
