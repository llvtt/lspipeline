package main

import (
	"fmt"
	"github.com/TreyBastian/colourize"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/aws/aws-sdk-go/service/codepipeline/codepipelineiface"
	"github.com/jedib0t/go-pretty/v6/table"
	"os"
	"time"
)

type lspipeline struct {
	client codepipelineiface.CodePipelineAPI
}

const dateFormat = "15:04:05 02-01-2006 PT"

func NewLsPipeline() *lspipeline {
	s, err := session.NewSession()
	if err != nil {
		panic(err)
	}
	c := codepipeline.New(s)
	return &lspipeline{c}
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

var statusColors = map[string]int{
	"Succeeded": colourize.Green,
}

func (app *lspipeline) printPipelineStatus(pipelineName string) error {
	pipelineState, err := app.client.GetPipelineState(&codepipeline.GetPipelineStateInput{
		Name: aws.String(pipelineName),
	})
	if err != nil {
		return err
	}

	now := time.Now()

	tw := table.NewWriter()
	tw.SetOutputMirror(os.Stdout)
	tw.AppendHeader(table.Row{"Stage Name", "Status", "Last Change"})
	for _, state := range pipelineState.StageStates {
		latestExecution := state.ActionStates[0].LatestExecution
		executionStatus := *latestExecution.Status

		tw.AppendRow(table.Row{
			*state.StageName,
			colourize.Colourize(executionStatus, statusColors[executionStatus]),
			prettyPrintTime(&now, latestExecution.LastStatusChange),
		})
	}
	tw.Render()

	return nil
}

// pretty print time `t` relative to `now`
func prettyPrintTime(now, t *time.Time) string {
	diff := now.Sub(*t)
	minutes := int(diff.Minutes())
	seconds := int(diff.Seconds())

	if minutes >= 30 {
		return t.In(time.Local).Format(dateFormat)
	} else if minutes > 0 {
		return fmt.Sprintf("%i minutes, %i seconds ago", minutes, seconds)
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
	if err := app.printPipelineStatus(pipelineName); err != nil {
		panic(err)
	}
}

func main() {
	NewLsPipeline().Run()
}
