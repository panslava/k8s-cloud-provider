/*
Copyright 2023 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package exec

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/google/go-cmp/cmp"
)

// testAction is used for unit testing.
type testAction struct {
	ActionBase
	name   string
	events []Event
	err    error
}

func (a *testAction) String() string {
	return fmt.Sprintf("%s(%v)", a.name, a.events)
}

func (a *testAction) DryRun() []Event {
	return a.events
}

func (a *testAction) Run(context.Context, cloud.Cloud) ([]Event, error) {
	return a.events, a.err
}

func (a *testAction) Metadata() *ActionMetadata {
	return &ActionMetadata{
		Name:    fmt.Sprintf("%s(%v)", a.name, a.events),
		Type:    ActionTypeCustom,
		Summary: "Action used for testing",
	}
}

func TestActionBase(t *testing.T) {
	for _, tc := range []struct {
		name    string
		events  []Event
		signals []Event

		wantSignalRet []bool
		wantPending   []Event
		wantDone      []Event
		wantCanRun    bool
	}{
		{
			name:          "signal one event",
			events:        []Event{StringEvent("a")},
			signals:       []Event{StringEvent("a")},
			wantSignalRet: []bool{true},
			wantPending:   []Event{},
			wantDone:      []Event{StringEvent("a")},
			wantCanRun:    true,
		},
		{
			name:          "signal one event ignored",
			events:        []Event{StringEvent("a")},
			signals:       []Event{StringEvent("b")},
			wantSignalRet: []bool{false},
			wantPending:   []Event{StringEvent("a")},
			wantDone:      []Event{},
			wantCanRun:    false,
		},
		{
			name:          "multiple events out of order",
			events:        []Event{StringEvent("a"), StringEvent("b")},
			signals:       []Event{StringEvent("b"), StringEvent("a")},
			wantSignalRet: []bool{true, true},
			wantPending:   []Event{},
			wantDone:      []Event{StringEvent("a"), StringEvent("b")},
			wantCanRun:    true,
		},
		{
			name:          "multiple events pending",
			events:        []Event{StringEvent("a"), StringEvent("b")},
			signals:       []Event{StringEvent("b"), StringEvent("c")},
			wantSignalRet: []bool{true, false},
			wantPending:   []Event{StringEvent("a")},
			wantDone:      []Event{StringEvent("b")},
			wantCanRun:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotSignalRet []bool
			ab := ActionBase{Want: tc.events}
			for _, e := range tc.signals {
				gotSignalRet = append(gotSignalRet, ab.Signal(e))
			}
			if diff := cmp.Diff(gotSignalRet, tc.wantSignalRet); diff != "" {
				t.Errorf("gotSignalRet diff: -got/+want: %s", diff)
			}
			if diff := diffEvents(ab.PendingEvents(), tc.wantPending); diff != "" {
				t.Errorf("ab.Want diff: -got/+want: %s", diff)
			}
			if diff := diffEvents(ab.Done, tc.wantDone); diff != "" {
				t.Errorf("ab.Done diff: -got/+want: %s", diff)
			}
			if got := ab.CanRun(); got != tc.wantCanRun {
				t.Errorf("ab.CanRun() = %t, want %t", got, tc.wantCanRun)
			}
		})
	}
}

func TestEventAction(t *testing.T) {
	resID := &cloud.ResourceID{
		ProjectID: "proj1",
		Resource:  "res1",
		Key:       meta.GlobalKey("x"),
	}
	ev := NewExistsAction(resID)
	type values struct {
		CanRan    bool
		Signal    bool
		S         string
		Pending   []Event
		RunEvents []Event
	}

	got := values{
		CanRan:  ev.CanRun(),
		Signal:  ev.Signal(StringEvent("ev1")),
		S:       ev.String(),
		Pending: ev.PendingEvents(),
	}

	var err error
	got.RunEvents, err = ev.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("ev.Run() = %v, want nil", err)
	}
	diff := cmp.Diff(got, values{
		CanRan:    true,
		Signal:    false,
		S:         "EventAction([Exists(res1:proj1/x)])",
		Pending:   nil,
		RunEvents: []Event{&existsEvent{id: resID}},
	})
	if diff != "" {
		t.Errorf("diff: -got/+want: %s", diff)
	}
}
