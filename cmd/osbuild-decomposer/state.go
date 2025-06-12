package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"

	"github.com/google/uuid"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
)

// BuildRequest represents a build request, it
type BuildRequest struct {
	Id  uuid.UUID
	Req *v2.ComposeRequest
}

type BuildStatus v2.ComposeStatus

type State struct {
	mu sync.Mutex

	current    *BuildRequest
	buildQueue []BuildRequest
}

func NewState() *State {
	return &State{}
}

func (s *State) AddBuildRequest(ctx context.Context, req BuildRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buildQueue = append(s.buildQueue, req)
	s.maybeBuildNext(ctx)
}

func (s *State) Builds(ctx context.Context) []BuildStatus {
	var res []BuildStatus

	matches, err := filepath.Glob(baseOutputDir + "/*")
	if err != nil {
		slog.Error("cannot list composes", "error", err)
		return nil
	}
	for _, m := range matches {
		id := filepath.Base(m)
		res = append(res, s.BuildStatus(ctx, id))
	}
	return res
}

func (s *State) BuildStatus(ctx context.Context, id string) (res BuildStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res.Id = id
	res.Kind = "ComposeStatus"

	buildResult := NewBuildResult(filepath.Join(baseOutputDir, id))
	switch {
	case buildResult.Bad():
		res.Status = v2.ComposeStatusValueFailure
		res.ImageStatus.Status = v2.ImageStatusValueFailure
	case buildResult.Good():
		res.Status = v2.ComposeStatusValueSuccess
		res.ImageStatus.Status = v2.ImageStatusValueSuccess
	case buildResult.Unknown():
		// check if it is the current build
		if s.current != nil && s.current.Id.String() == id {
			res.Status = v2.ComposeStatusValuePending
			res.ImageStatus.Status = v2.ImageStatusValueBuilding
			return
		}
		// check if in the queue
		inBuildQueue := slices.ContainsFunc(s.buildQueue, func(br BuildRequest) bool {
			return br.Id.String() == id
		})
		if inBuildQueue {
			res.Status = v2.ComposeStatusValuePending
			res.ImageStatus.Status = v2.ImageStatusValuePending
			return
		}
		// not building, not in build queue, no status assume
		// something bad happend (e.g. a reboot) during build
		res.Status = v2.ComposeStatusValueFailure
		res.ImageStatus.Status = v2.ImageStatusValueFailure
	}

	return res
}

func (s *State) maybeBuildNext(ctx context.Context) {
	if s.current != nil {
		return
	}
	if len(s.buildQueue) == 0 {
		return
	}

	s.current = &s.buildQueue[0]
	s.buildQueue = s.buildQueue[1:]

	outputDir := filepath.Join(baseOutputDir, s.current.Id.String())
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		slog.Error("compose dir creation failed", "error", err)
		return
	}

	go func() {
		err := runIbcli(ctx, outputDir, s.current.Req)
		if err != nil {
			slog.Error("running ibcli failed", "error", err)
		}
		if err1 := NewBuildResult(outputDir).Mark(err); err1 != nil {
			slog.Error("error setting build result", "error", err1)
		}

		// queue the next build
		s.mu.Lock()
		defer s.mu.Unlock()
		s.current = nil
		s.maybeBuildNext(ctx)
	}()

}

var ibcliBinary = "image-builder"

func runIbcli(ctx context.Context, outputDir string, req *v2.ComposeRequest) error {
	// XXX: put into its own validate helper
	if req.ImageRequests != nil && len(*req.ImageRequests) > 1 {
		return fmt.Errorf("cannot handle more than one image req")
	}
	if req.Customizations != nil {
		return fmt.Errorf("cannot handle customizations, got %+v", req.Customizations)
	}
	if req.Koji != nil {
		return fmt.Errorf("cannot handle koji, got %+v", req.Koji)
	}
	var bp v2.Blueprint
	if req.Blueprint != nil {
		bp = *req.Blueprint
	}
	bpPath := filepath.Join(outputDir, "bp.json")
	f, err := os.Create(bpPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(bp); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("cannot fsync() %q", f.Name())
	}

	cmd := exec.Command(
		ibcliBinary,
		"build",
		"-v",
		"--with-buildlog",
		"--blueprint", bpPath,
		"--output-dir", outputDir,
		"--distro", req.Distribution,
		string(req.ImageRequest.ImageType),
		// XXX: handle architecutures, repositories?
	)

	// output to stdout for easier logs via the journal
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = outputDir
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// BuildResult is a simple wrapper to store a build result on disk
// XXX: consider jsondb for better compat with osbuild-composer
// on-prem
type BuildResult struct {
	resultGood string
	resultBad  string
}

func NewBuildResult(buildDir string) *BuildResult {
	return &BuildResult{
		resultGood: filepath.Join(buildDir, "result.good"),
		resultBad:  filepath.Join(buildDir, "result.bad"),
	}
}

func (br *BuildResult) Mark(err error) error {
	if err == nil {
		return os.WriteFile(br.resultGood, nil, 0600)
	} else {
		return os.WriteFile(br.resultBad, nil, 0600)
	}
}

func (br *BuildResult) Good() bool {
	_, err := os.Stat(br.resultGood)
	return err == nil
}

func (br *BuildResult) Bad() bool {
	_, err := os.Stat(br.resultBad)
	return err == nil
}

func (br *BuildResult) Unknown() bool {
	return !br.Good() && !br.Bad()
}
