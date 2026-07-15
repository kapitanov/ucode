package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tui"
)

type memoryStorage struct {
	*jsonStorage[iface.Memory]
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{
		jsonStorage: newJSONStorage(
			"memory",
			func(_ *iface.Memory) {},
			func() *iface.Memory {
				return &iface.Memory{
					Items: []iface.MemoryItem{},
				}
			},
		),
	}
}

type planStorage interface {
	Get() *iface.Plan
	Save() error
}

type persistedPlanStorage struct {
	*jsonStorage[iface.Plan]
}

func newPersistedPlanStorage() planStorage {
	return &persistedPlanStorage{
		jsonStorage: newJSONStorage(
			"plan",
			func(plan *iface.Plan) {
				for i := range plan.Items {
					plan.Items[i].Index = i + 1
				}
			},
			func() *iface.Plan {
				return &iface.Plan{
					Items: []iface.PlanItem{},
				}
			},
		),
	}
}

type transientPlanStorage struct {
	plan iface.Plan
}

func newTransientPlanStorage() planStorage {
	return &transientPlanStorage{
		plan: iface.Plan{
			Items: []iface.PlanItem{},
		},
	}
}

func (s *transientPlanStorage) Get() *iface.Plan { return &s.plan }
func (*transientPlanStorage) Save() error        { return nil }

type jsonStorage[T any] struct {
	path  string
	value *T
}

func newJSONStorage[T any](name string, postLoadFunc func(*T), defaultFunc func() *T) *jsonStorage[T] {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	path := filepath.Join(wd, ".agents", name+".json")
	s := &jsonStorage[T]{path: path}
	s.value = s.Load()
	if s.value == nil {
		s.value = defaultFunc()
	}
	postLoadFunc(s.value)
	return s
}

func (s *jsonStorage[T]) Get() *T { return s.value }

func (s *jsonStorage[T]) Save() error {
	bs, err := json.MarshalIndent(s.value, "", "    ")
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(s.path), 0755)
	if err != nil {
		return err
	}

	err = os.WriteFile(s.path, bs, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (s *jsonStorage[T]) Load() *T {
	bs, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		panic(err)
	}

	var data T
	err = json.Unmarshal(bs, &data)
	if err != nil {
		panic(err)
	}

	tui.Printf("%% Loaded %q", s.path)
	return &data
}

func writeSubagentCallFile(agent iface.Agent, request string) error {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	path := filepath.Join(wd, ".agents", fmt.Sprintf("%s.md", agent.Name()))
	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("# %s\n\n%s\n", agent.Name(), request)

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return err
	}

	return nil
}
