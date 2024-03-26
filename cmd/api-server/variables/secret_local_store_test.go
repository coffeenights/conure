package variables

import (
	"testing"
)

func TestLocalSecretKeyStorage_Generate(t *testing.T) {
	type fields struct {
		filepath string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:    "test",
			fields:  fields{filepath: "test"},
			wantErr: false,
		}, {
			name:    "test fail",
			fields:  fields{filepath: "test/1.key"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLocalSecretKey(tt.fields.filepath)
			if err := l.Generate(); (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLocalSecretKeyStorage_Load(t *testing.T) {
	type fields struct {
		filepath string
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name:    "test",
			fields:  fields{filepath: "test"},
			want:    nil,
			wantErr: false,
		},
		{
			name:    "test fail",
			fields:  fields{filepath: "fake"},
			want:    []byte{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLocalSecretKey(tt.fields.filepath)
			got, err := l.Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil && tt.wantErr == false {
				t.Errorf("Load() got = %v, want %v", got, tt.want)
			}
		})
	}
}
