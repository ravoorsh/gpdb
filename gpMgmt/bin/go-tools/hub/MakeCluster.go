package hub

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/greenplum-db/gp-common-go-libs/gplog"
	"github.com/greenplum-db/gpdb/gp/idl"
	"github.com/greenplum-db/gpdb/gp/utils"
	"github.com/greenplum-db/gpdb/gp/utils/greenplum"
	"golang.org/x/exp/maps"
)

func (s *Server) MakeCluster(request *idl.MakeClusterRequest, stream idl.Hub_MakeClusterServer) error {
	gplog.Debug("Starting MakeCluster")

	streamLogMsg(stream, "Starting MakeCluster")
	err := s.ValidateEnvironment(stream, request)
	if err != nil {
		gplog.Error("Error during validation:%v", err)
		return err
	}

	streamLogMsg(stream, "Creating coordinator segment")
	err = s.CreateAndStartCoordinator(request.GpArray.Coordinator, request.ClusterParams)
	if err != nil {
		return err
	}
	streamLogMsg(stream, "Successfully created coordinator segment")

	streamLogMsg(stream, "Starting to register primary segments with the coordinator")
	conn, err := greenplum.ConnectDatabase(request.GpArray.Coordinator.HostName, int(request.GpArray.Coordinator.Port))
	if err != nil {
		return fmt.Errorf("error connecting to database : %v", err)
	}
	
	defer conn.Close()
	err = greenplum.RegisterCoordinator(request.GpArray.Coordinator, conn)
	if err != nil {
		return err
	}

	err = greenplum.RegisterPrimaries(request.GpArray.Primaries, conn)
	if err != nil {
		return err
	}
	streamLogMsg(stream, "Successfully registered primary segments with the coordinator")

	gpArray := greenplum.NewGpArray()
	err = gpArray.ReadGpSegmentConfig(conn)
	if err != nil {
		return err
	}

	primarySegs, err := gpArray.GetPrimarySegments()
	if err != nil {
		return err
	}

	streamLogMsg(stream, "Creating primary segments")
	err = s.CreateSegments(stream, primarySegs, request.ClusterParams, []string{})
	if err != nil {
		return err
	}
	streamLogMsg(stream, "Successfully created primary segments")

	// We do not start the primary segments, only the coordinator.
	// Only in case of mirrors, we start them at the end after gpstop/gpstart
	// err = StartSegments(s.Conns, primarySegs, "-c gp_role=utility")

	streamLogMsg(stream, "Restarting the Greenplum cluster in production mode")
	gpstopOptions := &greenplum.GpStop{
		DataDirectory:   request.GpArray.Coordinator.DataDirectory,
		CoordinatorOnly: true,
	}
	cmd := utils.NewGpSourcedCommand(gpstopOptions, s.GpHome)
	err = streamExecCommand(stream, cmd, s.GpHome)
	if err != nil {
		return fmt.Errorf("executing gpstop: %w", err)
	}

	gpstartOptions := &greenplum.GpStart{
		DataDirectory: request.GpArray.Coordinator.DataDirectory,
	}
	cmd = utils.NewGpSourcedCommand(gpstartOptions, s.GpHome)
	err = streamExecCommand(stream, cmd, s.GpHome)
	if err != nil {
		return fmt.Errorf("executing gpstart: %w", err)
	}
	streamLogMsg(stream, "Completed restart of Greenplum cluster in production mode")

	// TODO
	// 1. CREATE_GPEXTENSIONS
	// 2. IMPORT_COLLATION
	// 3. CREATE_DATABASE
	// 4. SET_GP_USER_PW

	return err
}
func (s *Server) ValidateEnvironment(stream idl.Hub_MakeClusterServer, request *idl.MakeClusterRequest) error {
	gparray := request.GpArray
	hostDirMap := make(map[string][]string)
	// Add coordinator to the map
	hostDirMap[gparray.Coordinator.HostAddress] = append(hostDirMap[gparray.Coordinator.HostAddress], gparray.Coordinator.DataDirectory)

	// Add primaries to the map
	for _, seg := range gparray.Primaries {
		hostDirMap[seg.HostAddress] = append(hostDirMap[seg.HostAddress], seg.DataDirectory)
	}
	progressLabel := "Validating Hosts:"
	progressTotal := len(hostDirMap)
	streamProgressMsg(stream, progressLabel, progressTotal)
	validateFn := func(conn *Connection) error {
		dirList := hostDirMap[conn.Hostname]
		validateReq := idl.ValidateHostEnvRequest{DirectoryList: dirList, Forced: request.ForceFlag}
		_, err := conn.AgentClient.ValidateHostEnv(context.Background(), &validateReq)
		if err != nil {
			return err
		}
		streamProgressMsg(stream, progressLabel, progressTotal)
		return nil
	}
	return ExecuteRPC(s.Conns, validateFn)
}

func CreateSingleSegment(conn *Connection, seg *idl.Segment, clusterParams *idl.ClusterParams, addressList []string) error {
	pgConfig := make(map[string]string)
	maps.Copy(pgConfig, clusterParams.CommonConfig)
	if seg.Contentid == -1 {
		maps.Copy(pgConfig, clusterParams.CoordinatorConfig)
	} else {
		maps.Copy(pgConfig, clusterParams.SegmentConfig)
	}

	makeSegmentReq := &idl.MakeSegmentRequest{
		Segment:      seg,
		Locale:       clusterParams.Locale,
		Encoding:     clusterParams.Encoding,
		SegConfig:    pgConfig,
		IPList:       addressList,
		HbaHostNames: clusterParams.HbaHostnames,
	}

	_, err := conn.AgentClient.MakeSegment(context.Background(), makeSegmentReq)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) CreateAndStartCoordinator(seg *idl.Segment, clusterParams *idl.ClusterParams) error {
	coordinatorConn := getConnByHost(s.Conns, []string{seg.HostName})

	seg.Contentid = -1
	seg.Dbid = 1
	request := func(conn *Connection) error {
		err := CreateSingleSegment(conn, seg, clusterParams, []string{})
		if err != nil {
			return err
		}

		startSegReq := &idl.StartSegmentRequest{
			DataDir: seg.DataDirectory,
			Wait:    true,
			Options: "-c gp_role=utility",
		}
		_, err = conn.AgentClient.StartSegment(context.Background(), startSegReq)

		return err
	}

	return ExecuteRPC(coordinatorConn, request)
}

func (s *Server) CreateSegments(stream idl.Hub_MakeClusterServer, segs []greenplum.Segment, clusterParams *idl.ClusterParams, addressList []string) error {
	hostSegmentMap := map[string][]*idl.Segment{}
	for _, seg := range segs {
		segReq := &idl.Segment{
			Port:          int32(seg.Port),
			DataDirectory: seg.DataDirectory,
			HostName:      seg.HostName,
			HostAddress:   seg.HostAddress,
			Contentid:     int32(seg.ContentId),
			Dbid:          int32(seg.Dbid),
		}

		if _, ok := hostSegmentMap[seg.HostName]; !ok {
			hostSegmentMap[seg.HostName] = []*idl.Segment{segReq}
		} else {
			hostSegmentMap[seg.HostName] = append(hostSegmentMap[seg.HostName], segReq)
		}
	}

	progressLabel := "Initializing segments:"
	progressTotal := len(segs)
	streamProgressMsg(stream, progressLabel, progressTotal)

	request := func(conn *Connection) error {
		var wg sync.WaitGroup

		segs := hostSegmentMap[conn.Hostname]
		errs := make(chan error, len(segs))
		for _, seg := range segs {
			seg := seg
			wg.Add(1)
			go func(seg *idl.Segment) {
				defer wg.Done()

				err := CreateSingleSegment(conn, seg, clusterParams, addressList)
				if err != nil {
					errs <- utils.FormatGrpcError(err)
				} else {
					streamProgressMsg(stream, progressLabel, progressTotal)
				}
			}(seg)
		}

		wg.Wait()
		close(errs)

		var err error
		for e := range errs {
			err = errors.Join(err, e)
		}
		return err
	}

	return ExecuteRPC(s.Conns, request)
}

func StartSegments(conns []*Connection, segs []greenplum.Segment, options string) error {
	hostSegmentMap := map[string][]greenplum.Segment{}
	for _, seg := range segs {
		if _, ok := hostSegmentMap[seg.HostName]; !ok {
			hostSegmentMap[seg.HostName] = []greenplum.Segment{seg}
		} else {
			hostSegmentMap[seg.HostName] = append(hostSegmentMap[seg.HostName], seg)
		}
	}

	request := func(conn *Connection) error {
		var wg sync.WaitGroup

		segs := hostSegmentMap[conn.Hostname]
		errs := make(chan error, len(segs))
		for _, seg := range segs {
			seg := seg
			startReq := &idl.StartSegmentRequest{
				DataDir: seg.DataDirectory,
				Wait:    true,
				Timeout: 600,
				Options: options,
			}
			wg.Add(1)
			go func(seg greenplum.Segment) {
				defer wg.Done()

				_, err := conn.AgentClient.StartSegment(context.Background(), startReq)
				if err != nil {
					errs <- utils.FormatGrpcError(err)
				}
			}(seg)
		}

		wg.Wait()
		close(errs)

		var err error
		for e := range errs {
			err = errors.Join(err, e)
		}
		return err
	}

	return ExecuteRPC(conns, request)
}
