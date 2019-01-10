package testupdate

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-common/convert/ptr"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/jinzhu/gorm"
	"sync"
	"sync/atomic"
	"time"
)

type DummyUpdateExecutor struct {
	db             *gorm.DB
	config         *configuration.Data
	NumberOfCalls  *uint64
	TimeToSleep    time.Duration
	waitGroup      *sync.WaitGroup
	ClusterService cluster.Service
}

func NewDummyUpdateExecutor(db *gorm.DB, config *configuration.Data) *DummyUpdateExecutor {
	return &DummyUpdateExecutor{db: db, config: config, NumberOfCalls: ptr.Uint64(0)}
}

func (e *DummyUpdateExecutor) Update(ctx context.Context, dbTenant *tenant.Tenant, user *auth.User, envTypes []environment.Type, allowSelfHealing bool) error {
	atomic.AddUint64(e.NumberOfCalls, 1)

	time.Sleep(e.TimeToSleep)
	if e.waitGroup != nil {
		e.waitGroup.Wait()
	}

	if e.ClusterService == nil {
		return fmt.Errorf("cluster service is not set")
	}
	tenantUpdater := controller.TenantUpdater{TenantRepository: tenant.NewDBService(e.db),
		Config:         e.config,
		ClusterService: e.ClusterService,
	}
	return tenantUpdater.Update(ctx, dbTenant, user, envTypes, allowSelfHealing)
}
