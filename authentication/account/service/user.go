package service

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-auth/application/service"
	"github.com/fabric8-services/fabric8-auth/authorization/token/manager"

	"github.com/fabric8-services/fabric8-auth/application/service/base"
	servicecontext "github.com/fabric8-services/fabric8-auth/application/service/context"
	"github.com/fabric8-services/fabric8-auth/authentication/account/repository"
	"github.com/fabric8-services/fabric8-auth/errors"
	"github.com/fabric8-services/fabric8-auth/log"
	"github.com/satori/go.uuid"
)

// NewUserService creates a new service to manage users
func NewUserService(ctx servicecontext.ServiceContext) service.UserService {
	return &userServiceImpl{
		BaseService: base.NewBaseService(ctx),
	}
}

// userServiceImpl implements the UserService to manage users
type userServiceImpl struct {
	base.BaseService
	tokenManager manager.TokenManager
}

// UserInfo gets user information given a context containing access_token
func (s *userServiceImpl) UserInfo(ctx context.Context, identityID uuid.UUID) (*repository.User, *repository.Identity, error) {
	var identity *repository.Identity
	err := s.ExecuteInTransaction(func() error {
		var err error
		identity, err = s.Repositories().Identities().LoadWithUser(ctx, identityID)
		if err != nil || identity == nil {
			return errors.NewUnauthorizedError(fmt.Sprintf("auth token contains id %s of unknown Identity\n", identityID))
		}
		return nil
	})

	if err != nil {
		return nil, nil, err
	}
	log.Debug(ctx, map[string]interface{}{
		"identity_id": identity.ID,
		"user_id":     identity.User.ID,
	}, "loaded identity and user")
	return &identity.User, identity, nil
}

func (s *userServiceImpl) DeprovisionUser(ctx context.Context, username string) (*repository.Identity, error) {

	var identity *repository.Identity
	err := s.ExecuteInTransaction(func() error {

		identities, err := s.Repositories().Identities().Query(
			repository.IdentityWithUser(),
			repository.IdentityFilterByUsername(username),
			repository.IdentityFilterByProviderType(repository.KeycloakIDP))
		if err != nil {
			return err
		}
		if len(identities) == 0 {
			return errors.NewNotFoundErrorWithKey("user identity", "username", username)
		}

		identity = &identities[0]
		identity.User.Deprovisioned = true

		return s.Repositories().Users().Save(ctx, &identity.User)
	})

	return identity, err
}

// ContextIdentityIfExists returns the identity's ID found in given context if the identity exists in the Auth DB
// If it doesn't exist then an Unauthorized error is returned
func (s *userServiceImpl) ContextIdentityIfExists(ctx context.Context) (uuid.UUID, error) {
	identity, err := manager.ContextIdentity(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	// Check if the identity exists
	err = s.ExecuteInTransaction(func() error {
		err := s.Repositories().Identities().CheckExists(ctx, identity.String())
		if err != nil {
			return errors.NewUnauthorizedError(err.Error())
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return *identity, nil
}

// LoadContextIdentityAndUser returns the identity found in given context if the identity exists in the Auth DB
// If no token present in the context then an Unauthorized error is returned
// If the identity represented by the token doesn't exist in the DB or not associated with any User then an Unauthorized error is returned
func (s *userServiceImpl) LoadContextIdentityAndUser(ctx context.Context) (*repository.Identity, error) {
	var identity *repository.Identity
	identityID, err := manager.ContextIdentity(ctx)
	if err != nil {
		return nil, errors.NewUnauthorizedError(err.Error())
	}
	// Check if the identity exists
	identity, err = s.Repositories().Identities().LoadWithUser(ctx, *identityID)
	if err != nil {
		return nil, errors.NewUnauthorizedError(err.Error())
	}

	return identity, err
}

// LoadContextIdentityIfNotDeprovisioned returns the same identity as LoadContextIdentityAndUser()
// if the user is not deprovisioned. Returns an Unauthorized error if the user is deprovisioned.
func (s *userServiceImpl) LoadContextIdentityIfNotDeprovisioned(ctx context.Context) (*repository.Identity, error) {
	identity, err := s.LoadContextIdentityAndUser(ctx)
	if err != nil {
		return nil, err
	}
	if identity.User.Deprovisioned {
		return nil, errors.NewUnauthorizedError("user deprovisioned")
	}
	return identity, err
}
