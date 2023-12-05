package diskfill

import (
	"reflect"
	"strings"
	"testing"
)

func Test_calculateDiskUsage(t *testing.T) {
	type args struct {
		lines string
	}
	tests := []struct {
		name    string
		args    args
		want    DiskUsage
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				lines: `Filesystem      Mounted on      Type    File            1K-blocks     Avail    Used
						/dev/nvme0n1p1  /disk-fill-temp overlay /disk-fill-temp 61252480   60228480 1024000`,
			},
			want: DiskUsage{
				Capacity:  61252480,
				Used:      1024000,
				Available: 60228480,
			},
			wantErr: false,
		},{
			name: "success 2",
			args: args{
				lines: `Filesystem      Mounted on      Type    File            1K-blocks     Available    Used
						/dev/nvme0n1p1  /disk-fill-temp overlay /disk-fill-temp 61252480   60228480 1024000`,
			},
			want: DiskUsage{
				Capacity:  61252480,
				Used:      1024000,
				Available: 60228480,
			},
			wantErr: false,
		}, {
			name: "success 3",
			args: args{
				lines: `Filesystem   1024-blocks      Used Available Capacity  Mounted on
/dev/disk3s5   971350180 885772704  69225984    93%    /System/Volumes/Data`,
			},
			want: DiskUsage{
				Capacity:  971350180,
				Used:      885772704,
				Available: 69225984,
			},
			wantErr: false,
		},  {
			name: "success 4",
			args: args{
				lines: `Filesystem           1024-blocks    Used Available Capacity Mounted on
overlay               61202244  37121180  20939740  64% /`,
			},
			want: DiskUsage{
				Capacity:  61202244,
				Used:      37121180,
				Available: 20939740,
			},
			wantErr: false,
		}, {
			name: "success 5",
			args: args{
				lines: `1024-blocks Filesystem Available Used Capacity Mounted on
61202244 overlay                  20939740 37121180  64% /`,
			},
			want: DiskUsage{
				Capacity:  61202244,
				Used:      37121180,
				Available: 20939740,
			},
			wantErr: false,
		}, {
			name: "fail",
			args: args{
				lines: `Filesystem      Mounted on      Type    File            512K-blocks    Avail    Used
						/dev/nvme0n1p1  /disk-fill-temp overlay /disk-fill-temp 119634      60228480 1024000`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateDiskUsage(strings.NewReader(tt.args.lines))
			if (err != nil) != tt.wantErr {
				t.Errorf("calculateDiskUsage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("calculateDiskUsage() got = %v, want %v", got, tt.want)
			}
		})
	}
}
