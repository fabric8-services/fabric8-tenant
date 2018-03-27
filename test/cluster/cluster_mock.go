package cluster

/*
DO NOT EDIT!
This code was generated automatically using github.com/gojuno/minimock v1.9
The original interface "Service" can be found in github.com/fabric8-services/fabric8-tenant/cluster
*/
import (
	context "context"
	"sync/atomic"
	"time"

	cluster "github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/gojuno/minimock"

	testify_assert "github.com/stretchr/testify/assert"
)

//ServiceMock implements github.com/fabric8-services/fabric8-tenant/cluster.Service
type ServiceMock struct {
	t minimock.Tester

	GetClustersFunc       func(p context.Context) (r []*cluster.Cluster, r1 error)
	GetClustersCounter    uint64
	GetClustersPreCounter uint64
	GetClustersMock       mServiceMockGetClusters

	StatsFunc       func() (r cluster.Stats)
	StatsCounter    uint64
	StatsPreCounter uint64
	StatsMock       mServiceMockStats

	StopFunc       func()
	StopCounter    uint64
	StopPreCounter uint64
	StopMock       mServiceMockStop
}

//NewServiceMock returns a mock for github.com/fabric8-services/fabric8-tenant/cluster.Service
func NewServiceMock(t minimock.Tester) *ServiceMock {
	m := &ServiceMock{t: t}

	if controller, ok := t.(minimock.MockController); ok {
		controller.RegisterMocker(m)
	}

	m.GetClustersMock = mServiceMockGetClusters{mock: m}
	m.StatsMock = mServiceMockStats{mock: m}
	m.StopMock = mServiceMockStop{mock: m}

	return m
}

type mServiceMockGetClusters struct {
	mock             *ServiceMock
	mockExpectations *ServiceMockGetClustersParams
}

//ServiceMockGetClustersParams represents input parameters of the Service.GetClusters
type ServiceMockGetClustersParams struct {
	p context.Context
}

//Expect sets up expected params for the Service.GetClusters
func (m *mServiceMockGetClusters) Expect(p context.Context) *mServiceMockGetClusters {
	m.mockExpectations = &ServiceMockGetClustersParams{p}
	return m
}

//Return sets up a mock for Service.GetClusters to return Return's arguments
func (m *mServiceMockGetClusters) Return(r []*cluster.Cluster, r1 error) *ServiceMock {
	m.mock.GetClustersFunc = func(p context.Context) ([]*cluster.Cluster, error) {
		return r, r1
	}
	return m.mock
}

//Set uses given function f as a mock of Service.GetClusters method
func (m *mServiceMockGetClusters) Set(f func(p context.Context) (r []*cluster.Cluster, r1 error)) *ServiceMock {
	m.mock.GetClustersFunc = f
	return m.mock
}

//GetClusters implements github.com/fabric8-services/fabric8-tenant/cluster.Service interface
func (m *ServiceMock) GetClusters(p context.Context) (r []*cluster.Cluster, r1 error) {
	atomic.AddUint64(&m.GetClustersPreCounter, 1)
	defer atomic.AddUint64(&m.GetClustersCounter, 1)

	if m.GetClustersMock.mockExpectations != nil {
		testify_assert.Equal(m.t, *m.GetClustersMock.mockExpectations, ServiceMockGetClustersParams{p},
			"Service.GetClusters got unexpected parameters")

		if m.GetClustersFunc == nil {

			m.t.Fatal("No results are set for the ServiceMock.GetClusters")

			return
		}
	}

	if m.GetClustersFunc == nil {
		m.t.Fatal("Unexpected call to ServiceMock.GetClusters")
		return
	}

	return m.GetClustersFunc(p)
}

//GetClustersMinimockCounter returns a count of ServiceMock.GetClustersFunc invocations
func (m *ServiceMock) GetClustersMinimockCounter() uint64 {
	return atomic.LoadUint64(&m.GetClustersCounter)
}

//GetClustersMinimockPreCounter returns the value of ServiceMock.GetClusters invocations
func (m *ServiceMock) GetClustersMinimockPreCounter() uint64 {
	return atomic.LoadUint64(&m.GetClustersPreCounter)
}

type mServiceMockStats struct {
	mock *ServiceMock
}

//Return sets up a mock for Service.Stats to return Return's arguments
func (m *mServiceMockStats) Return(r cluster.Stats) *ServiceMock {
	m.mock.StatsFunc = func() cluster.Stats {
		return r
	}
	return m.mock
}

//Set uses given function f as a mock of Service.Stats method
func (m *mServiceMockStats) Set(f func() (r cluster.Stats)) *ServiceMock {
	m.mock.StatsFunc = f
	return m.mock
}

//Stats implements github.com/fabric8-services/fabric8-tenant/cluster.Service interface
func (m *ServiceMock) Stats() (r cluster.Stats) {
	atomic.AddUint64(&m.StatsPreCounter, 1)
	defer atomic.AddUint64(&m.StatsCounter, 1)

	if m.StatsFunc == nil {
		m.t.Fatal("Unexpected call to ServiceMock.Stats")
		return
	}

	return m.StatsFunc()
}

//StatsMinimockCounter returns a count of ServiceMock.StatsFunc invocations
func (m *ServiceMock) StatsMinimockCounter() uint64 {
	return atomic.LoadUint64(&m.StatsCounter)
}

//StatsMinimockPreCounter returns the value of ServiceMock.Stats invocations
func (m *ServiceMock) StatsMinimockPreCounter() uint64 {
	return atomic.LoadUint64(&m.StatsPreCounter)
}

type mServiceMockStop struct {
	mock *ServiceMock
}

//Return sets up a mock for Service.Stop to return Return's arguments
func (m *mServiceMockStop) Return() *ServiceMock {
	m.mock.StopFunc = func() {
		return
	}
	return m.mock
}

//Set uses given function f as a mock of Service.Stop method
func (m *mServiceMockStop) Set(f func()) *ServiceMock {
	m.mock.StopFunc = f
	return m.mock
}

//Stop implements github.com/fabric8-services/fabric8-tenant/cluster.Service interface
func (m *ServiceMock) Stop() {
	atomic.AddUint64(&m.StopPreCounter, 1)
	defer atomic.AddUint64(&m.StopCounter, 1)

	if m.StopFunc == nil {
		m.t.Fatal("Unexpected call to ServiceMock.Stop")
		return
	}

	m.StopFunc()
}

//StopMinimockCounter returns a count of ServiceMock.StopFunc invocations
func (m *ServiceMock) StopMinimockCounter() uint64 {
	return atomic.LoadUint64(&m.StopCounter)
}

//StopMinimockPreCounter returns the value of ServiceMock.Stop invocations
func (m *ServiceMock) StopMinimockPreCounter() uint64 {
	return atomic.LoadUint64(&m.StopPreCounter)
}

//ValidateCallCounters checks that all mocked methods of the interface have been called at least once
//Deprecated: please use MinimockFinish method or use Finish method of minimock.Controller
func (m *ServiceMock) ValidateCallCounters() {

	if m.GetClustersFunc != nil && atomic.LoadUint64(&m.GetClustersCounter) == 0 {
		m.t.Fatal("Expected call to ServiceMock.GetClusters")
	}

	if m.StatsFunc != nil && atomic.LoadUint64(&m.StatsCounter) == 0 {
		m.t.Fatal("Expected call to ServiceMock.Stats")
	}

	if m.StopFunc != nil && atomic.LoadUint64(&m.StopCounter) == 0 {
		m.t.Fatal("Expected call to ServiceMock.Stop")
	}

}

//CheckMocksCalled checks that all mocked methods of the interface have been called at least once
//Deprecated: please use MinimockFinish method or use Finish method of minimock.Controller
func (m *ServiceMock) CheckMocksCalled() {
	m.Finish()
}

//Finish checks that all mocked methods of the interface have been called at least once
//Deprecated: please use MinimockFinish or use Finish method of minimock.Controller
func (m *ServiceMock) Finish() {
	m.MinimockFinish()
}

//MinimockFinish checks that all mocked methods of the interface have been called at least once
func (m *ServiceMock) MinimockFinish() {

	if m.GetClustersFunc != nil && atomic.LoadUint64(&m.GetClustersCounter) == 0 {
		m.t.Fatal("Expected call to ServiceMock.GetClusters")
	}

	if m.StatsFunc != nil && atomic.LoadUint64(&m.StatsCounter) == 0 {
		m.t.Fatal("Expected call to ServiceMock.Stats")
	}

	if m.StopFunc != nil && atomic.LoadUint64(&m.StopCounter) == 0 {
		m.t.Fatal("Expected call to ServiceMock.Stop")
	}

}

//Wait waits for all mocked methods to be called at least once
//Deprecated: please use MinimockWait or use Wait method of minimock.Controller
func (m *ServiceMock) Wait(timeout time.Duration) {
	m.MinimockWait(timeout)
}

//MinimockWait waits for all mocked methods to be called at least once
//this method is called by minimock.Controller
func (m *ServiceMock) MinimockWait(timeout time.Duration) {
	timeoutCh := time.After(timeout)
	for {
		ok := true
		ok = ok && (m.GetClustersFunc == nil || atomic.LoadUint64(&m.GetClustersCounter) > 0)
		ok = ok && (m.StatsFunc == nil || atomic.LoadUint64(&m.StatsCounter) > 0)
		ok = ok && (m.StopFunc == nil || atomic.LoadUint64(&m.StopCounter) > 0)

		if ok {
			return
		}

		select {
		case <-timeoutCh:

			if m.GetClustersFunc != nil && atomic.LoadUint64(&m.GetClustersCounter) == 0 {
				m.t.Error("Expected call to ServiceMock.GetClusters")
			}

			if m.StatsFunc != nil && atomic.LoadUint64(&m.StatsCounter) == 0 {
				m.t.Error("Expected call to ServiceMock.Stats")
			}

			if m.StopFunc != nil && atomic.LoadUint64(&m.StopCounter) == 0 {
				m.t.Error("Expected call to ServiceMock.Stop")
			}

			m.t.Fatalf("Some mocks were not called on time: %s", timeout)
			return
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

//AllMocksCalled returns true if all mocked methods were called before the execution of AllMocksCalled,
//it can be used with assert/require, i.e. assert.True(mock.AllMocksCalled())
func (m *ServiceMock) AllMocksCalled() bool {

	if m.GetClustersFunc != nil && atomic.LoadUint64(&m.GetClustersCounter) == 0 {
		return false
	}

	if m.StatsFunc != nil && atomic.LoadUint64(&m.StatsCounter) == 0 {
		return false
	}

	if m.StopFunc != nil && atomic.LoadUint64(&m.StopCounter) == 0 {
		return false
	}

	return true
}
