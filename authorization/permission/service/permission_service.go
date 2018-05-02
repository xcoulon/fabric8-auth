package service

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-auth/application/repository"
	"github.com/fabric8-services/fabric8-auth/errors"
	"github.com/satori/go.uuid"
)

type PermissionService interface {
	HasScope(ctx context.Context, identityID uuid.UUID, resourceID string, scopeName string) (bool, error)
	RequireScope(ctx context.Context, identityID uuid.UUID, resourceID string, scopeName string) error
}

// PermissionServiceImpl is the implementation of the interface for
// PermissionModelService. IMPORTANT NOTE: Transaction control is not provided by this service
type PermissionServiceImpl struct {
	repos repository.Repositories
}

// NewPermissionModelService creates a new service.
func NewPermissionService(repositories repository.Repositories) PermissionService {
	return &PermissionServiceImpl{repos: repositories}
}

// HasScope does a permission check for a user, to determine whether they have a particular scope for the
// specified resource.  It does this by executing a rather complex query against the database, which checks whether the
// user, or any of the identity groups (i.e. teams, organizations, security groups) that it is a member of has been
// assigned a role that grants the specified scope.  It takes into account resource hierarchies, checking the roles of
// parent and other ancestor resources, and also takes into account role mappings, which allow roles assigned for a
// certain type of resource in the resource ancestry to map to a role for a different resource type lower in the
// resource hierarchy.
func (s *PermissionServiceImpl) HasScope(ctx context.Context, identityID uuid.UUID, resourceID string, scopeName string) (bool, error) {

	identityRoles, err := s.repos.IdentityRoleRepository().FindPermissions(ctx, identityID, resourceID, scopeName)
	if err != nil {
		return false, err
	}

	return len(identityRoles) > 0, nil
}

// RequireScope is the same as HasScope, except instead of returning a boolean value it will just return an error if the
// identity does not have the specified scope for the resource
func (s *PermissionServiceImpl) RequireScope(ctx context.Context, identityID uuid.UUID, resourceID string, scopeName string) error {
	result, err := s.HasScope(ctx, identityID, resourceID, scopeName)
	if err != nil {
		return errors.NewInternalError(ctx, err)
	}

	if !result {
		return errors.NewForbiddenError(fmt.Sprintf("identity with ID %s does not have required scope %s for resource %s", identityID.String(), scopeName, resourceID))
	}

	return nil
}
