package testupdate

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-common/convert/ptr"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"sync"
	"sync/atomic"
	"time"
)

type DummyUpdateExecutor struct {
	NumberOfCalls             *uint64
	TimeToSleep               time.Duration
	ShouldFail                bool
	waitGroup                 *sync.WaitGroup
	ShouldCallOriginalUpdater bool
}

func NewDummyUpdateExecutor() *DummyUpdateExecutor {
	return &DummyUpdateExecutor{NumberOfCalls: ptr.Uint64(0)}
}

func (e *DummyUpdateExecutor) Update(ctx context.Context, tenantService tenant.Service, openshiftConfig openshift.Config, t *tenant.Tenant,
	envTypes []environment.Type, usertoken string, allowSelfHealing bool) (map[environment.Type]string, error) {

	atomic.AddUint64(e.NumberOfCalls, 1)

	time.Sleep(e.TimeToSleep)
	if e.waitGroup != nil {
		e.waitGroup.Wait()
	}
	if e.ShouldCallOriginalUpdater {
		return controller.TenantUpdater{}.Update(ctx, tenantService, openshiftConfig, t, envTypes, usertoken, allowSelfHealing)
	}
	if e.ShouldFail {
		return testdoubles.GetMappedVersions(envTypes...), fmt.Errorf("failing")
	}
	return testdoubles.GetMappedVersions(envTypes...), nil
}
