package sync

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/open-integration/core"
	upsert "github.com/open-integration/service-catalog/google-spreadsheet/pkg/endpoints/upsert"
	"github.com/spf13/viper"
)

type (
	// Handler - exposed struct that implementd Handler interface
	Handler struct{}

	Row struct {
		ID   string   `json:"ID"`
		Data []string `json:"Data"`
	}

	TrelloCard struct {
		ID               string      `json:"id"`
		IDShort          float64     `json:"idShort"`
		Name             string      `json:"name"`
		Pos              float64     `json:"pos"`
		Email            string      `json:"email"`
		ShortLink        string      `json:"shortLink"`
		ShortURL         string      `json:"shortUrl"`
		URL              string      `json:"url"`
		Desc             string      `json:"desc"`
		Due              interface{} `json:"due"`
		DueComplete      bool        `json:"dueComplete"`
		Closed           bool        `json:"closed"`
		Subscribed       bool        `json:"subscribed"`
		DateLastActivity time.Time   `json:"dateLastActivity"`
		Board            interface{} `json:"Board"`
		IDBoard          string      `json:"idBoard"`
		List             struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			IDBoard    string `json:"idBoard"`
			Closed     bool   `json:"closed"`
			Pos        int    `json:"pos"`
			Subscribed bool   `json:"subscribed"`
		} `json:"List"`
		IDList string `json:"idList"`
		Badges struct {
			Votes              int  `json:"votes"`
			ViewingMemberVoted bool `json:"viewingMemberVoted"`
			Subscribed         bool `json:"subscribed"`
			CheckItems         int  `json:"checkItems"`
			CheckItemsChecked  int  `json:"checkItemsChecked"`
			Comments           int  `json:"comments"`
			Attachments        int  `json:"attachments"`
			Description        bool `json:"description"`
		} `json:"badges"`
		IDCheckLists          []interface{} `json:"idCheckLists"`
		IDAttachmentCover     string        `json:"idAttachmentCover"`
		ManualCoverAttachment bool          `json:"manualCoverAttachment"`
		IDLabels              []string      `json:"idLabels"`
		Labels                []struct {
			ID      string  `json:"id"`
			IDBoard string  `json:"idBoard"`
			Name    string  `json:"name"`
			Color   string  `json:"color"`
			Uses    float64 `json:"uses"`
		} `json:"labels"`
	}
)

// Handle - the function that will be called from the CLI with viper config
// to provide access to all flags
func (g *Handler) Handle(cnf *viper.Viper) error {
	p := build(cnf)
	opt := &core.EngineOptions{
		Pipeline: *p,
	}
	if cnf.GetString("kubernetesKubeconfigPath") != "" && cnf.GetString("kubernetesNamespace") != "" && cnf.GetString("kubernetesContext") != "" {
		opt.Kubeconfig = &core.EngineKubernetesOptions{
			Path:      cnf.GetString("kubernetesKubeconfigPath"),
			Context:   cnf.GetString("kubernetesContext"),
			Namespace: cnf.GetString("kubernetesNamespace"),
		}
	}

	if cnf.GetBool("kubernetesInCluster") {
		namespace := "default"
		if cnf.GetString("kubernetesNamespace") != "" {
			namespace = cnf.GetString("kubernetesNamespace")
		}
		opt.Kubeconfig = &core.EngineKubernetesOptions{
			InCluster: true,
			Namespace: namespace,
		}
	}

	e := core.NewEngine(opt)
	return e.Run()
}

func build(cnf *viper.Viper) *core.Pipeline {
	return &core.Pipeline{
		Metadata: core.PipelineMetadata{
			Name: "sync-trello",
		},
		Spec: buildPipelineSpec(cnf),
	}
}

func buildPipelineSpec(cnf *viper.Viper) core.PipelineSpec {
	return core.PipelineSpec{
		Services: []core.Service{
			core.Service{
				Name:    "trello",
				Version: "0.9.0",
				As:      "TrelloSVC",
			},
			core.Service{
				Name:    "google-spreadsheet",
				Version: "0.10.0",
				As:      "GoogleSVC",
			},
		},
		Tasks: []core.Task{
			core.Task{
				Metadata: buildTaskMetadata("Fetch Cards From Trello"),
				Condition: &core.Condition{
					Name: "Engine started",
					Func: conditionOnEngineStarted(),
				},
				Spec: buildSpecTaskTrelloSync(cnf.GetString("trelloAppKey"), cnf.GetString("trelloToken"), cnf.GetString("trelloBoardId")),
			},
			core.Task{
				Metadata: buildTaskMetadata("Update Google Spreadsheet"),
				Condition: &core.Condition{
					Name: "Fetch Cards From Trello Finished",
					Func: conditionTaskFinishedWithStatus("Fetch Cards From Trello", core.TaskStatusSuccess),
				},
				SpecFunc: buildSpecFuncGoogleRowsUpsert(cnf.GetString("googleServiceAccount"), cnf.GetString("googleSpreadsheetId")),
			},
			core.Task{
				Metadata: buildTaskMetadata("Archive sync cards"),
				Condition: &core.Condition{
					Name: "Upsert rows completed successfuly",
					Func: conditionTaskFinishedWithStatus("Update Google Spreadsheet", core.TaskStatusSuccess),
				},
				SpecFunc: buildSpecFunncArchiveTrelloCards(cnf.GetString("trelloAppKey"), cnf.GetString("trelloToken"), cnf.GetString("trelloBoardId")),
			},
		},
	}
}

func buildTaskMetadata(name string) core.TaskMetadata {
	return core.TaskMetadata{
		Name: name,
	}
}

func buildTask(name string, condition *core.Condition, spec *core.TaskSpec) core.Task {
	return core.Task{
		Metadata:  buildTaskMetadata(name),
		Condition: condition,
		Spec:      *spec,
	}
}

func conditionOnEngineStarted() func(ev *core.Event, state *core.State) bool {
	return func(ev *core.Event, state *core.State) bool {
		return ev.Metadata.Name == "engine.started"
	}
}

func conditionTaskFinished(task string) func(ev *core.Event, state *core.State) bool {
	return func(ev *core.Event, state *core.State) bool {
		for _, t := range state.Tasks {
			if t.State == "finished" && t.Task == task {
				return true
			}
		}
		return false
	}
}

func conditionTaskFinishedWithStatus(task string, status string) func(ev *core.Event, state *core.State) bool {
	return func(ev *core.Event, state *core.State) bool {
		for _, t := range state.Tasks {
			if t.Status == status && t.State == "finished" && t.Task == task {
				return true
			}
		}
		return false
	}
}

func load(j string) ([]*TrelloCard, error) {
	cards := []*TrelloCard{}
	err := json.Unmarshal([]byte(j), &cards)
	if err != nil {
		return nil, err
	}
	return cards, nil
}

func buildSpecTaskTrelloSync(trelloAppKey string, trelloToken string, trelloBoardID string) core.TaskSpec {
	return core.TaskSpec{
		Service:  "TrelloSVC",
		Endpoint: "getcards",
		Arguments: []core.Argument{
			core.Argument{
				Key:   "App",
				Value: trelloAppKey,
			},
			core.Argument{
				Key:   "Token",
				Value: trelloToken,
			},
			core.Argument{
				Key:   "Board",
				Value: trelloBoardID,
			},
		},
	}
}

func buildSpecFuncGoogleRowsUpsert(googleServiceAccount string, googleSpreadsheetID string) func(state *core.State) (*core.TaskSpec, error) {
	f, err := ioutil.ReadFile(googleServiceAccount)
	if err != nil {
		return func(state *core.State) (*core.TaskSpec, error) {
			return nil, err
		}
	}
	sa := &upsert.ServiceAccount{}
	err = json.Unmarshal(f, &sa)
	if err != nil {
		return func(state *core.State) (*core.TaskSpec, error) {
			return nil, err
		}
	}
	return func(state *core.State) (*core.TaskSpec, error) {
		output := ""
		for _, t := range state.Tasks {
			if t.Task == "Fetch Cards From Trello" {
				output = t.Output
			}
		}
		cards, err := load(output)
		if err != nil {
			return nil, err
		}
		rows := []*Row{}
		for _, c := range cards {
			labels := []string{}
			for _, l := range c.Labels {
				labels = append(labels, l.Name)
			}
			now := time.Now()
			createdAt := now.AddDate(0, 0, -1).Format("02-01-2006 15:04:05")

			id := strconv.Itoa(int(c.IDShort))
			rows = append(rows, &Row{
				ID: id,
				Data: []string{
					createdAt,
					c.DateLastActivity.Format("02-01-2006 15:04:05"),
					c.Name,
					c.ShortURL,
					strings.Join(labels, " "),
					c.List.Name,
				},
			})
		}
		return &core.TaskSpec{
			Service:  "GoogleSVC",
			Endpoint: "upsert",
			Arguments: []core.Argument{
				core.Argument{
					Key:   "Rows",
					Value: rows,
				},
				core.Argument{
					Key:   "ServiceAccount",
					Value: sa,
				},
				core.Argument{
					Key:   "SpreadsheetID",
					Value: googleSpreadsheetID,
				},
			},
		}, nil
	}
}

func buildSpecFunncArchiveTrelloCards(trelloAppKey string, trelloToken string, trelloBoardID string) func(state *core.State) (*core.TaskSpec, error) {
	return func(state *core.State) (*core.TaskSpec, error) {
		output := ""
		for _, t := range state.Tasks {
			if t.Task == "Fetch Cards From Trello" {
				output = t.Output
			}
		}
		cards, err := load(output)
		if err != nil {
			return nil, err
		}
		cardids := []string{}
		for _, card := range cards {
			if card.List.Name == "Done" {
				cardids = append(cardids, card.ID)
			}
		}
		return &core.TaskSpec{
			Service:  "TrelloSVC",
			Endpoint: "archivecards",
			Arguments: []core.Argument{
				core.Argument{
					Key:   "App",
					Value: trelloAppKey,
				},
				core.Argument{
					Key:   "Token",
					Value: trelloToken,
				},
				core.Argument{
					Key:   "Board",
					Value: trelloBoardID,
				},
				core.Argument{
					Key:   "CardIDs",
					Value: cardids,
				},
			},
		}, nil
	}
}
