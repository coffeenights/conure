package variables

import "testing"

func TestIsValid(t *testing.T) {
	tests := []struct {
		name string
		vt   VariableType
		want bool
	}{
		{
			name: "test",
			vt:   "organization",
			want: true,
		},
		{
			name: "test fail",
			vt:   "fake",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vt.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
