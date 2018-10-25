package factory

import (
	"fmt"
	"time"

	factorymanager "github.com/fabric8-services/fabric8-auth/application/factory/manager"
	"github.com/fabric8-services/fabric8-auth/application/repository"
	"github.com/fabric8-services/fabric8-auth/application/service"
	"github.com/fabric8-services/fabric8-auth/application/service/context"
	"github.com/fabric8-services/fabric8-auth/application/transaction"
	"github.com/fabric8-services/fabric8-auth/configuration"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/pkg/errors"
)

type serviceContextImpl struct {
	repositories              repository.Repositories
	transactionalRepositories repository.Repositories
	transactionManager        transaction.TransactionManager
	inTransaction             bool
	services                  service.Services
	factories                 service.Factories
}

func NewServiceContext(repos repository.Repositories, tm transaction.TransactionManager, config *configuration.ConfigurationData, options ...Option) context.ServiceContext {
	ctx := new(serviceContextImpl)
	ctx.repositories = repos
	ctx.transactionManager = tm
	ctx.inTransaction = false

	var sc context.ServiceContext
	sc = ctx
	ctx.factories = factorymanager.NewManager(func() context.ServiceContext { return sc }, config)
	ctx.services = NewServiceFactory(func() context.ServiceContext { return sc }, config, options...)
	return ctx
}

func (s *serviceContextImpl) Repositories() repository.Repositories {
	if s.inTransaction {
		return s.transactionalRepositories
	} else {
		return s.repositories
	}
}

func (s *serviceContextImpl) Factories() service.Factories {
	return s.factories
}

func (s *serviceContextImpl) Services() service.Services {
	return s.services
}

func (s *serviceContextImpl) ExecuteInTransaction(todo func() error) error {
	if !s.inTransaction {
		// If we are not in a transaction already, start a new transaction
		var tx transaction.Transaction
		var err error
		if tx, err = s.transactionManager.BeginTransaction(); err != nil {
			log.Error(nil, map[string]interface{}{
				"err": err,
			}, "database BeginTransaction failed!")

			return errors.WithStack(err)
		}

		// Set the transaction flag to true
		s.inTransaction = true

		// Set the transactional repositories property
		s.transactionalRepositories = tx.(repository.Repositories)

		defer s.endTransaction()

		return func() error {
			errorChan := make(chan error, 1)
			txTimeout := time.After(transaction.DatabaseTransactionTimeout())

			go func() {
				defer func() {
					if err := recover(); err != nil {
						errorChan <- errors.New(fmt.Sprintf("Unknown error: %v", err))
					}
				}()
				errorChan <- todo()
			}()

			select {
			case err := <-errorChan:
				if err != nil {
					log.Debug(nil, nil, "Rolling back the transaction...")
					tx.Rollback()
					log.Error(nil, map[string]interface{}{
						"err": err,
					}, "database transaction failed!")
					return errors.WithStack(err)
				}

				tx.Commit()
				log.Debug(nil, nil, "Commit the transaction!")
				return nil
			case <-txTimeout:
				log.Debug(nil, nil, "Rolling back the transaction...")
				tx.Rollback()
				log.Error(nil, nil, "database transaction timeout!")
				return errors.New("database transaction timeout!")
			}
		}()
	} else {
		// If we are in a transaction, simply execute the passed function
		return todo()
	}
}

func (s *serviceContextImpl) endTransaction() {
	s.inTransaction = false
}
