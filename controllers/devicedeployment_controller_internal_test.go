package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeDeloymentData(t *testing.T) {
	data := []struct {
		StartDeviceID       int32
		EndDeviceID         *int32
		MessageFrequency    int32
		DevicePerDeployment int32
		TotalDevices        *int32
	}{
		{
			StartDeviceID:       1,
			EndDeviceID:         int32P(4),
			MessageFrequency:    1,
			DevicePerDeployment: 1,
		},
		{
			StartDeviceID:       1,
			MessageFrequency:    1,
			DevicePerDeployment: 1,
			TotalDevices:        int32P(4),
		},
		{
			StartDeviceID:       1,
			EndDeviceID:         int32P(5),
			DevicePerDeployment: 3,
			MessageFrequency:    1,
		},
	}

	d := computeDeploymentData(data[0].StartDeviceID, data[0].EndDeviceID, data[0].DevicePerDeployment, data[0].TotalDevices, data[0].MessageFrequency)
	assert.Len(t, d, 4)

	d = computeDeploymentData(data[1].StartDeviceID, data[1].EndDeviceID, data[1].DevicePerDeployment, data[1].TotalDevices, data[1].MessageFrequency)
	assert.Len(t, d, 4)

	d = computeDeploymentData(data[2].StartDeviceID, data[2].EndDeviceID, data[2].DevicePerDeployment, data[2].TotalDevices, data[2].MessageFrequency)
	assert.Len(t, d, 2)
}

func int32P(v int32) *int32 {
	return &v
}
