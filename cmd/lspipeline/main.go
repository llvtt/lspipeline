package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/aws/aws-sdk-go/service/codepipeline/codepipelineiface"
	"github.com/rivo/tview"
	"os"
	"strings"
	"time"
)

type lspipeline struct {
	*tview.Application
	flex *tview.Flex

	pipeline codepipelineiface.CodePipelineAPI
}

const dateFormat = "15:04:05 02-01-2006 PT"
const refreshPeriod = 2*time.Second

func NewLsPipeline() *lspipeline {
	s, err := session.NewSession()
	if err != nil {
		panic(err)
	}
	c := codepipeline.New(s)
	return &lspipeline{tview.NewApplication(), nil, c}
}

func (app *lspipeline) pipelineNames() (pipelineNames []string) {
	err := app.pipeline.ListPipelinesPages(
		new(codepipeline.ListPipelinesInput),
		func(output *codepipeline.ListPipelinesOutput, lastPage bool) (getNext bool) {
			for _, summary := range output.Pipelines {
				pipelineNames = append(pipelineNames, *summary.Name)
			}
			return !lastPage
		})
	if err != nil {
		panic(err)
	}
	return
}

func (app *lspipeline) renderPipeline(pipelineName string) {
	for range time.Tick(refreshPeriod) {
		pipelineState, err := app.pipeline.GetPipelineState(&codepipeline.GetPipelineStateInput{
			Name: aws.String(pipelineName),
		})
		if err != nil {
			panic(err)
		}

		app.QueueUpdateDraw(func() {
			app.flex.Clear()

			now := time.Now()

			for _, state := range pipelineState.StageStates {
				app.renderPipelineStage(&now, state)
			}
		})
	}
}

func (app *lspipeline) renderPipelineActionState(rowFlex *tview.Flex, now *time.Time, state *codepipeline.ActionState) {
	view := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter).SetWrap(true).SetWordWrap(true)
	view.SetTitle(*state.ActionName).SetBorder(true)
	rowFlex.AddItem(view, 0, 1, false)

	_, _, width, _ := app.flex.GetInnerRect()
	compact := width < 30

	latestExecution := state.LatestExecution
	var text strings.Builder
	if latestExecution == nil {
		text.WriteString("no latest execution")
	} else {
		text.WriteString("Last Status Change: ")
		if compact {
			text.WriteString("\n")
		}
		text.WriteString(prettyPrintTime(now, state.LatestExecution.LastStatusChange))
		text.WriteString("\n")
		text.WriteString("Status: ")
		text.WriteString(prettyPrintStatus(state.LatestExecution.Status))
	}

	_, err := view.Write([]byte(text.String()))
	if err != nil {
		panic(err)
	}
}

func (app *lspipeline) renderPipelineStage(now *time.Time, state *codepipeline.StageState) {
	rowFlex := tview.NewFlex()

	for _, state := range state.ActionStates {
		app.renderPipelineActionState(rowFlex, now, state)
	}

	app.flex.AddItem(rowFlex, 0, 1, false)
}

func (app *lspipeline) printPipelineNames() {
	for _, name := range app.pipelineNames() {
		fmt.Println(name)
	}
}

var statusColors = map[string]string{
	codepipeline.StageExecutionStatusSucceeded:  "green",
	codepipeline.StageExecutionStatusInProgress: "blue",
	codepipeline.StageExecutionStatusFailed:     "red",
	codepipeline.StageExecutionStatusCancelled:  "gray",
	codepipeline.StageExecutionStatusStopped:    "brown",
	codepipeline.StageExecutionStatusStopping:   "cyan",
}

const (
	styleBold = "b"
	styleBlink = "l"
	styleDim = "d"
	styleReverse = "r"
	styleUnderline = "u"
	styleReset = "-"
)

func tviewStyle(fg, bg string, style ...string) string {
	return fmt.Sprintf("[%s:%s:%s]", fg, bg, strings.Join(style, ""))
}

var resetAllStyles = tviewStyle("-", "-", "-")

// pretty print time `t` relative to `now`
func prettyPrintTime(now, t *time.Time) string {
	diff := now.Sub(*t)
	minutes := int(diff.Minutes())
	seconds := int(diff.Seconds()) % 60

	if minutes >= 30 {
		return t.In(time.Local).Format(dateFormat)
	} else if minutes > 0 {
		return fmt.Sprintf("%d minutes, %d seconds ago", minutes, seconds)
	} else {
		return fmt.Sprintf("%d seconds ago", int(diff.Seconds()))
	}
}

func prettyPrintStatus(status *string) string {
	color := statusColors[*status]
	style := tviewStyle(color, "-", styleBold)
	return strings.Join([]string{style, *status, resetAllStyles}, "")
}

func (app *lspipeline) Run() {
	if len(os.Args) < 2 {
		fmt.Printf(`USAGE:

	// Show pipeline status
	%s <pipeline-name>

	// List all pipeline names (no arguments)
	%s

`, os.Args[0], os.Args[0])
		fmt.Println("====== PIPELINES ======")
		app.printPipelineNames()
		os.Exit(0)
	}

	pipelineName := os.Args[1]

	if app.flex == nil {
		app.flex = tview.NewFlex().SetDirection(tview.FlexRow)
		app.SetRoot(app.flex, true)
	}
	go app.renderPipeline(pipelineName)
	if err := app.Application.Run(); err != nil {
		panic(err)
	}
}

func main() {
	NewLsPipeline().Run()
}
