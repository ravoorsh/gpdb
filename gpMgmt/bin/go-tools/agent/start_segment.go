package agent

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/greenplum-db/gpdb/gp/idl"
	"github.com/greenplum-db/gpdb/gp/utils"
	"github.com/greenplum-db/gpdb/gp/utils/postgres"
)

func (s *Server) StartSegment(ctx context.Context, in *idl.StartSegmentRequest) (*idl.StartSegmentReply, error) {
	pgCtlStartOptions := postgres.PgCtlStart{
		PgData: in.DataDir,
		Wait: in.Wait,
		Timeout: int(in.Timeout),
		Options: in.Options,
		Logfile: filepath.Join(in.DataDir, "log", "startup.log"),
	}
	out, err := utils.RunExecCommand(&pgCtlStartOptions, s.GpHome)
	if err != nil {
		return &idl.StartSegmentReply{}, fmt.Errorf("executing pg_ctl start: %s, %w", out, err)
	}

	return &idl.StartSegmentReply{}, nil
}
