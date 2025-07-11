// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH
//go:build !windows

package ociruntime

import (
	"reflect"
	"testing"
	"time"
)

func Test_unmarshalGuarded(t *testing.T) {
	type args struct {
		output []byte
	}

	timeVal, _ := time.Parse(time.RFC3339, "2023-09-20T05:35:15.520959889Z")
	container := ContainerState{
		ID:      "7d51145a4959742f7185563dc72f7fd9b08c6c375db406696ae0c94eac7f787e",
		Status:  "running",
		Bundle:  "/run/containerd/io.containerd.runtime.v2.task/moby/7d51145a4959742f7185563dc72f7fd9b08c6c375db406696ae0c94eac7f787e",
		Rootfs:  "/var/lib/docker/overlay2/88d42eefb3b59ff1055efa14e6ac07bffd30e3321242bc546bcf1e69b607f0b0/merged",
		Pid:     14907,
		Created: timeVal,
	}

	warning := "time=\"2023-09-20T19:36:27Z\" level=debug msg=\"openat2 not available, falling back to securejoin\" func=\"libcontainer/cgroups.prepareOpenat2.func1()\" file=\"libcontainer/cgroups/file.go:95\"\n"
	payload := "{\n  \"ociVersion\": \"1.0.2-dev\",\n  \"id\": \"7d51145a4959742f7185563dc72f7fd9b08c6c375db406696ae0c94eac7f787e\",\n  \"pid\": 14907,\n  \"status\": \"running\",\n  \"bundle\": \"/run/containerd/io.containerd.runtime.v2.task/moby/7d51145a4959742f7185563dc72f7fd9b08c6c375db406696ae0c94eac7f787e\",\n  \"rootfs\": \"/var/lib/docker/overlay2/88d42eefb3b59ff1055efa14e6ac07bffd30e3321242bc546bcf1e69b607f0b0/merged\",\n  \"created\": \"2023-09-20T05:35:15.520959889Z\",\n  \"owner\": \"\"\n}"
	tests := []struct {
		name    string
		args    args
		want    ContainerState
		wantErr bool
	}{
		{
			name: "json only output",
			args: args{
				output: []byte(payload),
			},
			want:    container,
			wantErr: false,
		},
		{
			name: "include warning",
			args: args{
				output: []byte(warning + payload),
			},
			want:    container,
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				output: []byte(warning),
			},
			want:    ContainerState{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var state ContainerState
			err := unmarshalGuarded(tt.args.output, &state)
			if (err != nil) != tt.wantErr {
				t.Errorf("unmarshalGuarded() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(state, tt.want) {
				t.Errorf("unmarshalGuarded() got = %v, want %v", state, tt.want)
			}
		})
	}
}
