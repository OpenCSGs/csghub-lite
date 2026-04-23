package cli

import "testing"

func TestNewRunCmdExposesContextFlags(t *testing.T) {
	cmd := newRunCmd()

	if f := cmd.Flags().Lookup("num-ctx"); f == nil {
		t.Fatal("expected --num-ctx flag")
	}
	if f := cmd.Flags().Lookup("num-parallel"); f == nil {
		t.Fatal("expected --num-parallel flag")
	}
}

func TestValidateInteractiveModelOverrides(t *testing.T) {
	tests := []struct {
		name        string
		numCtx      int
		numParallel int
		wantErr     bool
	}{
		{name: "unset", numCtx: 0, numParallel: 0},
		{name: "valid explicit overrides", numCtx: 131072, numParallel: 1},
		{name: "reject too small ctx", numCtx: 512, numParallel: 0, wantErr: true},
		{name: "reject negative parallel", numCtx: 0, numParallel: -1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInteractiveModelOverrides(tt.numCtx, tt.numParallel)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateInteractiveModelOverrides(%d, %d) error = %v, wantErr %v", tt.numCtx, tt.numParallel, err, tt.wantErr)
			}
		})
	}
}
