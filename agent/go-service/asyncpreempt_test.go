package main

import (
	"reflect"
	"testing"
)

func TestAsyncPreemptOffEnv(t *testing.T) {
	tests := []struct {
		name        string
		env         []string
		wantEnv     []string
		wantChanged bool
	}{
		{
			name:        "no GODEBUG entry appends one",
			env:         []string{"PATH=/usr/bin"},
			wantEnv:     []string{"PATH=/usr/bin", "GODEBUG=asyncpreemptoff=1"},
			wantChanged: true,
		},
		{
			name:        "empty GODEBUG is set directly",
			env:         []string{"GODEBUG="},
			wantEnv:     []string{"GODEBUG=asyncpreemptoff=1"},
			wantChanged: true,
		},
		{
			name:        "existing settings are preserved and appended",
			env:         []string{"GODEBUG=http2client=0"},
			wantEnv:     []string{"GODEBUG=http2client=0,asyncpreemptoff=1"},
			wantChanged: true,
		},
		{
			name:        "already disabled is left untouched",
			env:         []string{"PATH=/usr/bin", "GODEBUG=asyncpreemptoff=1"},
			wantEnv:     []string{"PATH=/usr/bin", "GODEBUG=asyncpreemptoff=1"},
			wantChanged: false,
		},
		{
			name:        "already disabled alongside other settings is left untouched",
			env:         []string{"GODEBUG=http2client=0,asyncpreemptoff=1"},
			wantEnv:     []string{"GODEBUG=http2client=0,asyncpreemptoff=1"},
			wantChanged: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotEnv, gotChanged := asyncPreemptOffEnv(test.env)
			if gotChanged != test.wantChanged || !reflect.DeepEqual(gotEnv, test.wantEnv) {
				t.Fatalf(
					"asyncPreemptOffEnv(%v) = (%v, %t), want (%v, %t)",
					test.env,
					gotEnv,
					gotChanged,
					test.wantEnv,
					test.wantChanged,
				)
			}
		})
	}
}
