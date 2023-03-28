package costmodel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencost/opencost/pkg/kubecost"
)

//go:generate moq -out moq_cloud_storage_test.go . CloudStorage:CloudStorageMock
//go:generate moq -out moq_allocation_model_test.go . AllocationModel:AllocationModelMock

func Test_UpdateCSV(t *testing.T) {
	t.Run("previous data doesn't exist, upload new data", func(t *testing.T) {
		storage := &CloudStorageMock{
			ExistsFunc: func(path string) (bool, error) {
				return false, nil
			},
			WriteFunc: func(name string, data []byte) error {
				return nil
			},
		}
		model := &AllocationModelMock{
			DateRangeFunc: func() (time.Time, time.Time, error) {
				return time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC), nil
			},
			ComputeAllocationFunc: func(start time.Time, end time.Time, resolution time.Duration) (*kubecost.AllocationSet, error) {
				return &kubecost.AllocationSet{
					Allocations: map[string]*kubecost.Allocation{
						"test": {
							Start:                  time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), // required for GPU metrics
							End:                    time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
							CPUCoreUsageAverage:    0.1,
							CPUCoreRequestAverage:  0.2,
							CPUCost:                0.3,
							RAMBytesUsageAverage:   0.4,
							RAMBytesRequestAverage: 0.5,
							RAMCost:                0.6,
							GPUHours:               48,
							GPUCost:                0.8,
							NetworkCost:            0.9,
							NetworkTransferBytes:   10,
							NetworkReceiveBytes:    11,
							PVs: map[kubecost.PVKey]*kubecost.PVAllocation{
								kubecost.PVKey{
									Cluster: "test-cluster",
									Name:    "test-pv",
								}: {
									ByteHours: 48,
									Cost:      2.0,
								},
							}, // 2 PVBytes, 2 PVCost
							Properties: &kubecost.AllocationProperties{
								Namespace:      "test-namespace",
								Controller:     "test-controller-name",
								ControllerKind: "test-controller-kind",
								Pod:            "test-pod",
								Container:      "test-container",
							},
						},
					},
				}, nil
			},
		}
		err := UpdateCSV(context.TODO(), storage, model, "/test.csv")
		require.NoError(t, err)
		// uploaded a single file with the data
		assert.Len(t, storage.WriteCalls(), 1)
		assert.Len(t, model.ComputeAllocationCalls(), 1)
		assert.Equal(t, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), model.ComputeAllocationCalls()[0].Start)
		assert.Equal(t, time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC), model.ComputeAllocationCalls()[0].End)
		assert.Equal(t, `Date,Namespace,ControllerKind,ControllerName,Pod,Container,CPUCoreUsageAverage,CPUCoreRequestAverage,RAMBytesUsageAverage,RAMBytesRequestAverage,NetworkReceiveBytes,NetworkTransferBytes,GPUs,PVBytes,CPUCost,RAMCost,NetworkCost,PVCost,GPUCost,TotalCost
2021-01-01,test-namespace,test-controller-kind,test-controller-name,test-pod,test-container,0.1,0.2,0.4,0.5,11,10,2,2,0.3,0.6,0.9,2,0.8,4.6000000000000005
`, string(storage.WriteCalls()[0].Data))
	})

	t.Run("merge new data with previous data (with different CSV structure)", func(t *testing.T) {
		storage := &CloudStorageMock{
			ExistsFunc: func(name string) (bool, error) {
				return true, nil
			},
			ReadFunc: func(name string) ([]byte, error) {
				return []byte(`Date,Namespace,CPUCoreUsageAverage,CPUCoreRequestAverage,CPUCost,RAMBytesUsageAverage,RAMBytesRequestAverage,RAMCost
2021-01-01,test-namespace,0.1,0.2,0.3,0.4,0.5,0.6
`), nil
			},
			WriteFunc: func(name string, data []byte) error {
				return nil
			},
		}
		model := &AllocationModelMock{
			DateRangeFunc: func() (time.Time, time.Time, error) {
				return time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC), nil
			},
			ComputeAllocationFunc: func(start time.Time, end time.Time, resolution time.Duration) (*kubecost.AllocationSet, error) {
				return &kubecost.AllocationSet{
					Allocations: map[string]*kubecost.Allocation{
						"test": {
							Properties: &kubecost.AllocationProperties{
								Namespace: "test-namespace",
							},
							CPUCost: 1,
						},
					},
				}, nil
			},
		}
		err := UpdateCSV(context.TODO(), storage, model, "/test.csv")
		require.NoError(t, err)
		// uploaded a single file with the data
		assert.Len(t, storage.WriteCalls(), 1)
		assert.Len(t, model.ComputeAllocationCalls(), 1)
		assert.Len(t, model.ComputeAllocationCalls(), 1)
		// 2021-01-01 is already in the export file, so we only compute for 2021-01-02
		assert.Equal(t, time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC), model.ComputeAllocationCalls()[0].Start)
		assert.Equal(t, time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC), model.ComputeAllocationCalls()[0].End)
		assert.Equal(t, `Date,Namespace,CPUCoreUsageAverage,CPUCoreRequestAverage,CPUCost,RAMBytesUsageAverage,RAMBytesRequestAverage,RAMCost,ControllerKind,ControllerName,Pod,Container,NetworkReceiveBytes,NetworkTransferBytes,GPUs,PVBytes,NetworkCost,PVCost,GPUCost,TotalCost
2021-01-01,test-namespace,0.1,0.2,0.3,0.4,0.5,0.6,,,,,,,,,,,,
2021-01-02,test-namespace,0,0,1,0,0,0,,,,,0,0,0,0,0,0,0,1
`, string(storage.WriteCalls()[0].Data))
	})

	t.Run("data already present in export file, export should be skipped", func(t *testing.T) {
		storage := &CloudStorageMock{
			ExistsFunc: func(name string) (bool, error) {
				return true, nil
			},
			ReadFunc: func(name string) ([]byte, error) {
				return []byte(`Date,Name,CPUCoreUsageAverage,CPUCoreRequestAverage,CPUCost,RAMBytesUsageAverage,RAMBytesRequestAverage,RAMCost
2021-01-01,test,0.1,0.2,0.3,0.4,0.5,0.6
`), nil
			},
		}
		model := &AllocationModelMock{
			DateRangeFunc: func() (time.Time, time.Time, error) {
				return time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC), nil
			},
		}
		err := UpdateCSV(context.TODO(), storage, model, "/test.csv")
		require.NoError(t, err)
		assert.Len(t, storage.WriteCalls(), 0)
		assert.Len(t, model.ComputeAllocationCalls(), 0)
	})

	t.Run("allocation data is empty", func(t *testing.T) {
		model := &AllocationModelMock{
			ComputeAllocationFunc: func(start time.Time, end time.Time, resolution time.Duration) (*kubecost.AllocationSet, error) {
				return &kubecost.AllocationSet{
					Allocations: nil,
				}, nil
			},
			DateRangeFunc: func() (time.Time, time.Time, error) {
				return time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC), nil
			},
		}
		storage := &CloudStorageMock{
			ExistsFunc: func(name string) (bool, error) {
				return false, nil
			},
		}
		err := UpdateCSV(context.TODO(), storage, model, "/test.csv")
		require.Equal(t, err, errNoData)
	})
}
