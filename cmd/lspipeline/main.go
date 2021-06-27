package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/aws/aws-sdk-go/service/codepipeline/codepipelineiface"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"os"
	"strings"
	"time"
)

type lspipeline struct {
	client codepipelineiface.CodePipelineAPI
	s tcell.Screen
}

const dateFormat = "15:04:05 02-01-2006 PT"
const refreshPeriod = 2 // seconds

func NewLsPipeline() *lspipeline {
	s, err := session.NewSession()
	if err != nil {
		panic(err)
	}
	c := codepipeline.New(s)
	return &lspipeline{c, nil}
}

func (app *lspipeline) initScreen() {
	if app.s == nil {
		var err error
		app.s, err = tcell.NewScreen()
		if err != nil {
			panic(err)
		}
		if err = app.s.Init(); err != nil {
			panic(err)
		}
	}
}

func (app *lspipeline) pipelineNames() (pipelineNames []string) {
	err := app.client.ListPipelinesPages(
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

func (app *lspipeline) printPipelineNames() {
	for _, name := range app.pipelineNames() {
		fmt.Println(name)
	}
}

var statusColors = map[string]tcell.Color{
	codepipeline.StageExecutionStatusSucceeded: tcell.ColorGreen,
	codepipeline.StageExecutionStatusInProgress: tcell.ColorBlue,
	codepipeline.StageExecutionStatusFailed: tcell.ColorRed,
	codepipeline.StageExecutionStatusCancelled: tcell.ColorSlateGray,
	codepipeline.StageExecutionStatusStopped: tcell.ColorBrown,
	codepipeline.StageExecutionStatusStopping: tcell.ColorSandyBrown,
}

func (app *lspipeline) renderPipelineStage(now *time.Time, top int, state *codepipeline.StageState) {
	latestExecution := state.ActionStates[0].LatestExecution
	executionStatus := *latestExecution.Status
	color := statusColors[executionStatus]

	lines := []string{
		*state.StageName,
		executionStatus,
		prettyPrintTime(now, latestExecution.LastStatusChange),
	}

	var longestLine int
	for _, line := range lines {
		if len(line) > longestLine {
			longestLine = len(line)
		}
	}

	w, _ := app.s.Size()
	left := (w - longestLine) / 2
	app.renderMessageBox(
		lines,
		left,
		top,
		tcell.StyleDefault.Foreground(color))
}

func (app *lspipeline) renderPipeline(pipelineName string) {
	pipelineState, err := app.client.GetPipelineState(&codepipeline.GetPipelineStateInput{
		Name: aws.String(pipelineName),
	})
	if err != nil {
		panic(err)
	}

	startTop := 1
	now := time.Now()
	for i, state := range pipelineState.StageStates {
		app.renderPipelineStage(&now, startTop + i*5, state)
	}

	_, h := app.s.Size()
	app.emitStr(0, h-1, tcell.StyleDefault, fmt.Sprintf("last updated %s", now.Format(dateFormat)))

	app.s.Show()
}

func (app *lspipeline) renderPipelineLoop(pipelineName string) {
	app.initScreen()

	ticker := time.NewTicker(time.Second * refreshPeriod)
	defer ticker.Stop()
	quit := make(chan bool)

	go func() {
		for {
			switch evt := app.s.PollEvent().(type) {
			case *tcell.EventKey:
				if evt.Key() == tcell.KeyEscape {
					close(quit)
					return
				}
			}
		}
	}()

	app.renderPipeline(pipelineName)
	for {
		select {
		case <- quit:
			app.s.Fini()
			return
		case <- ticker.C:
			app.renderPipeline(pipelineName)
		}
	}
}

// pretty print time `t` relative to `now`
func prettyPrintTime(now, t *time.Time) string {
	diff := now.Sub(*t)
	minutes := int(diff.Minutes())
	seconds := int(diff.Seconds())

	if minutes >= 30 {
		return t.In(time.Local).Format(dateFormat)
	} else if minutes > 0 {
		return fmt.Sprintf("%d minutes, %d seconds ago", minutes, seconds)
	} else {
		return fmt.Sprintf("%d seconds ago", int(diff.Seconds()))
	}
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
	app.renderPipelineLoop(pipelineName)
}

// Taken from https://github.com/gdamore/tcell/blob/master/_demos/hello_world.go
func (app *lspipeline) emitStr(x, y int, style tcell.Style, str string) {
	for _, c := range str {
		var comb []rune
		w := runewidth.RuneWidth(c)
		if w == 0 {
			comb = []rune{c}
			c = ' '
			w = 1
		}
		app.s.SetContent(x, y, c, comb, style)
		x += w
	}
}

const (
	tl = "╔"
	tr = "╗"
	vt = "║"
	hz = "═"
	bl = "╚"
	br = "╝"
)

func (app *lspipeline) renderMessageBox(lines []string, boxLeft, boxTop int, style tcell.Style) {
	// Figure out how to draw a box around "message"
	var boxWidth int
	boxSidePadding := 4  // 2 x (1 space on side, plus 1 for border)
	for _, line := range lines {
		if len(line) > boxWidth - boxSidePadding {
			boxWidth = len(line) + boxSidePadding
		}
	}

	// Construct top border
	var topBorder strings.Builder
	topBorder.WriteString(tl)
	for i := 0; i < boxWidth-2; i++ {
		topBorder.WriteString(hz)
	}
	topBorder.WriteString(tr)

	// Construct bottom border
	var bottomBorder strings.Builder
	bottomBorder.WriteString(bl)
	for i := 0; i < boxWidth-2; i++ {
		bottomBorder.WriteString(hz)
	}
	bottomBorder.WriteString(br)

	// Construct message lines (goes in the middle)
	var renderLines []string
	for _, line := range lines {
		var lineBuilder strings.Builder
		leftSpacePad := (boxWidth - len(line) - boxSidePadding) / 2 + 1
		rightSpacePad := boxWidth - len(line) - leftSpacePad - 2
		lineBuilder.WriteString(vt)
		for i := 0; i < leftSpacePad; i++ {
			lineBuilder.WriteString(" ")
		}
		lineBuilder.WriteString(line)
		for i := 0; i < rightSpacePad; i++ {
			lineBuilder.WriteString(" ")
		}
		lineBuilder.WriteString(vt)
		renderLines = append(renderLines, lineBuilder.String())
	}

	app.emitStr(boxLeft, boxTop, style, topBorder.String())
	for i, line := range renderLines {
		app.emitStr(boxLeft, boxTop+i+1, style, line)
	}
	app.emitStr(boxLeft, boxTop+len(renderLines)+1, style, bottomBorder.String())
}

func main() {
	NewLsPipeline().Run()
}
