package init_cluster

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greenplum-db/gpdb/gp/cli"
	"github.com/greenplum-db/gpdb/gp/test/integration/testutils"
)

func TestInitClusterCleanup(t *testing.T) {

	t.Run("check if the coordinator is stopped whenever an error occurs", func(t *testing.T) {

		var (
			valueSegPair []cli.SegmentPair
			okSeg        bool
			value        cli.Segment
			ok           bool
		)

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		primarySegs := config.Get("segment-array")
		if valueSegPair, okSeg = primarySegs.([]cli.SegmentPair); !okSeg {
			t.Fatalf("unexpected data type for segment-array %T", valueSegPair)
		}

		pSegPath := filepath.Join(valueSegPair[0].Primary.DataDirectory, "postgresql.conf")
		host := valueSegPair[0].Primary.Hostname
		coordinator := config.Get("coordinator")

		if value, ok = coordinator.(cli.Segment); !ok {
			t.Fatalf("unexpected data type for coordinator %T", value)
		}
		coordinatorDD := value.DataDirectory

		dirDeleted := make(chan bool)
		go func() {
			for {
				select {
				case <-dirDeleted:
					return
				default:
					cmdStr := fmt.Sprintf("if [ -f %s ]; then echo 'exists'; fi", pSegPath)
					cmd := exec.Command("ssh", host, cmdStr)
					output, err := cmd.Output()
					if err != nil {
						t.Errorf("unexpected error: %#v", err)
					}

					if strings.TrimSpace(string(output)) == "exists" {
						cmdStr := fmt.Sprintf("rm -rf %s", pSegPath)
						cmd := exec.Command("ssh", host, cmdStr)
						_, err := cmd.CombinedOutput()
						if err != nil {
							t.Errorf("unexpected error: %#v", err)
						}
						dirDeleted <- true
					}
					time.Sleep(1 * time.Second)
				}
			}
		}()

		result, err := testutils.RunInitCluster(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %s, %v", result.OutputMsg, err)
		}

		//Check if we got a message to rollback
		expectedOut := "[INFO]:-Please run gp init cluster --clean to rollback"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		//check if the coordinator is stopped
		cmd := exec.Command("pg_ctl", "status", "-D", coordinatorDD)
		cmdOuput, _ := cmd.CombinedOutput()
		expectedOut = "pg_ctl: no server running"
		if !strings.Contains(string(cmdOuput), expectedOut) {
			t.Fatalf("got %q, want %q", cmdOuput, expectedOut)
		}

		initResult, err := testutils.RunInitCluster("--clean")
		if err != nil {
			t.Fatalf("Error while cleaning cluster: %#v", err)
		}

		clusterExpectedOut := "[INFO]:-clean cluster command successful"
		if !strings.Contains(initResult.OutputMsg, clusterExpectedOut) {
			t.Errorf("got %q, want %q", initResult.OutputMsg, clusterExpectedOut)
		}

	})

	t.Run("when the mirror data directory is not empty", func(t *testing.T) {
		var ok bool
		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		mirrorSegs := config.Get("segment-array")
		valueSegPair, ok := mirrorSegs.([]cli.SegmentPair)

		if !ok {
			t.Fatalf("unexpected data type for segment-array %T", mirrorSegs)
		}

		MirrorHostName := valueSegPair[0].Mirror.Hostname
		cmdStr := fmt.Sprintf("mkdir -p %s && chmod 700 %s && touch %s/abc.txt", valueSegPair[0].Mirror.DataDirectory, valueSegPair[0].Mirror.DataDirectory, valueSegPair[0].Mirror.DataDirectory)

		cmdObj := exec.Command("ssh", MirrorHostName, cmdStr)
		_, errSeg := cmdObj.Output()
		if errSeg != nil {
			t.Fatalf("unexpected error : %v", errSeg)
		}

		result, err := testutils.RunInitCluster(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %s, %v", result.OutputMsg, err)
		}

		//Check if we got a message to rollback
		expectedOut := "[INFO]:-Please run gp init cluster --clean to rollback"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		expectedOut = fmt.Sprintf("[ERROR]:-Cluster creation failed, error host: %s, executing pg_basebackup: pg_basebackup: error: directory \"%s\" exists but is not empty\n", MirrorHostName, valueSegPair[0].Mirror.DataDirectory)
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		initResult, err := testutils.RunInitCluster("--clean")
		if err != nil {
			t.Fatalf("Error while cleaning cluster: %#v", err)
		}

		clusterExpectedOut := "[INFO]:-clean cluster command successful"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Errorf("got %q, want %q", initResult.OutputMsg, clusterExpectedOut)
		}

	})

	t.Run("cluster creation fails when primary is getting created, test if rollback is successful", func(t *testing.T) {
		var (
			ok           bool
			valueSegPair []cli.SegmentPair
			value        cli.Segment
			okSeg        bool
		)

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		primarySegs := config.Get("segment-array")
		if valueSegPair, okSeg = primarySegs.([]cli.SegmentPair); !okSeg {
			t.Fatalf("unexpected data type for segment-array %T", valueSegPair)
		}

		mSegPath := filepath.Join(valueSegPair[0].Mirror.DataDirectory, "postgresql.conf")
		host := valueSegPair[0].Mirror.Hostname
		coordinator := config.Get("coordinator")

		if value, ok = coordinator.(cli.Segment); !ok {
			t.Fatalf("unexpected data type for coordinator %T", value)
		}

		dirDeleted := make(chan bool)
		go func() {
			for {
				select {
				case <-dirDeleted:
					return
				default:
					cmdStr := fmt.Sprintf("if [ -f %s ]; then echo 'exists'; fi", mSegPath)
					cmd := exec.Command("ssh", host, cmdStr)
					output, err := cmd.Output()
					if err != nil {
						t.Errorf("unexpected error: %#v", err)
					}

					if strings.TrimSpace(string(output)) == "exists" {
						cmdStr := fmt.Sprintf("rm -rf %s", mSegPath)
						cmd := exec.Command("ssh", host, cmdStr)
						_, err := cmd.CombinedOutput()
						if err != nil {
							t.Errorf("unexpected error: %#v", err)
						}
						dirDeleted <- true
					}
					time.Sleep(1 * time.Second)
				}
			}
		}()

		result, err := testutils.RunInitCluster(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %s, %v", result.OutputMsg, err)
		}

		//Check if we got a message to rollback
		expectedOut := "[INFO]:-Please run gp init cluster --clean to rollback"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		//Test of --clean cluster is successful. This step is essential so that the subsequent run succeeds.
		initResult, err := testutils.RunInitCluster("--clean")
		if err != nil {
			t.Fatalf("Error while cleaning cluster: %#v", err)
		}

		clusterExpectedOut := "[INFO]:-clean cluster command successful"
		if !strings.Contains(initResult.OutputMsg, clusterExpectedOut) {
			t.Errorf("got %q, want %q", initResult.OutputMsg, clusterExpectedOut)
		}

		//Check the postgres processes are not running
		var hostlist []string
		hostlist = append(hostlist, value.Hostname)
		result, _ = testutils.CheckifClusterisRunning(hostlist)
		if result.ExitCode != 0 && strings.TrimSpace(result.OutputMsg) != "2" {
			t.Errorf("psotgres processes are still running")
		}

		//Check if the contents of data directory are removed
		hostDataDirMap := make(map[string][]string)
		hostDataDirMap[value.Hostname] = append(hostDataDirMap[value.Hostname], value.DataDirectory)
		result, _ = testutils.CheckifdataDirIsEmpty(hostDataDirMap)
		if result.ExitCode != 0 && strings.TrimSpace(result.OutputMsg) != "1" {
			t.Fatalf("data directory %s should be empty", value.DataDirectory)
		}

	})

	t.Run("cluster creation fails when mirror is getting created, test if rollback is successful", func(t *testing.T) {
		var ok bool

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		mirrorSegs := config.Get("segment-array")
		valueSegPair, ok := mirrorSegs.([]cli.SegmentPair)
		if !ok {
			t.Fatalf("unexpected data type for segment-array %T", mirrorSegs)
		}

		MirrorHostName := valueSegPair[0].Mirror.Hostname
		cmdStr := fmt.Sprintf("mkdir -p %s && chmod 700 %s && touch %s/abc.txt", valueSegPair[0].Mirror.DataDirectory, valueSegPair[0].Mirror.DataDirectory, valueSegPair[0].Mirror.DataDirectory)

		cmdObj := exec.Command("ssh", MirrorHostName, cmdStr)
		_, errSeg := cmdObj.Output()
		if errSeg != nil {
			t.Fatalf("unexpected error : %v", errSeg)
		}

		result, err := testutils.RunInitCluster(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %s, %v", result.OutputMsg, err)
		}

		//Check if we got a message to rollback
		expectedOut := "[INFO]:-Please run gp init cluster --clean to rollback"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		expectedOut = fmt.Sprintf("[ERROR]:-Cluster creation failed, error host: %s, executing pg_basebackup: pg_basebackup: error: directory \"%s\" exists but is not empty\n", MirrorHostName, valueSegPair[0].Mirror.DataDirectory)
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		//Run --clean after cluster init fails
		initResult, err := testutils.RunInitCluster("--clean")
		if err != nil {
			t.Fatalf("Error while cleaning cluster: %#v", err)
		}

		clusterExpectedOut := "[INFO]:-clean cluster command successful"
		if !strings.Contains(initResult.OutputMsg, clusterExpectedOut) {
			t.Errorf("got %q, want %q", initResult.OutputMsg, clusterExpectedOut)
		}

		coordinator := config.Get("coordinator")
		value, ok := coordinator.(cli.Segment)
		if !ok {
			t.Fatalf("unexpected data type for coordinator %T", value)
		}

		var hostlist []string
		hostlist = append(hostlist, value.Hostname)
		for _, seg := range valueSegPair {
			hostlist = append(hostlist, seg.Primary.Hostname)
		}

		//Verify there are no postgres processes running in co-ordinator and primary
		result, err = testutils.CheckifClusterisRunning(hostlist)
		if result.ExitCode != 0 && strings.TrimSpace(result.OutputMsg) != "2" {
			t.Errorf("postgres processes should not be running err: %v", err)
		}

		hostDataDirMap := make(map[string][]string)
		hostDataDirMap[value.Hostname] = append(hostDataDirMap[value.Hostname], value.DataDirectory)
		for _, seg := range valueSegPair {
			host := seg.Primary.Hostname
			dataDir := seg.Primary.DataDirectory
			hostDataDirMap[host] = append(hostDataDirMap[host], dataDir)

		}

		//Check if the contents of the directories are removed.
		result, _ = testutils.CheckifdataDirIsEmpty(hostDataDirMap)
		if result.ExitCode != 0 && strings.TrimSpace(result.OutputMsg) != "1" {
			t.Fatalf("data directory should be empty")
		}

	})
	t.Run("Init Cluster exits when --clean flag and --force flag is specified", func(t *testing.T) {

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}
		result, err := testutils.RunInitCluster("--force", "--clean")
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			t.Fatalf("got %v, want exit status 1", err)
		}

		expectedOut := "cannot use clean and force flag"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Errorf("got %q, want %q", result.OutputMsg, expectedOut)
		}
	})

	t.Run("Init Cluster exits when --clean flag and config file are specified", func(t *testing.T) {

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}
		result, err := testutils.RunInitCluster(configFile, "--clean")
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			t.Fatalf("got %v, want exit status 1", err)
		}

		expectedOut := "cannot provide config file with --clean flag"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Errorf("got %q, want %q", result.OutputMsg, expectedOut)
		}
	})

	t.Run("cluster creation fails, subsequent attempt to create cluster should fail and ask for rollback", func(t *testing.T) {
		var (
			ok           bool
			valueSegPair []cli.SegmentPair
			value        cli.Segment
			okSeg        bool
		)

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		primarySegs := config.Get("segment-array")
		if valueSegPair, okSeg = primarySegs.([]cli.SegmentPair); !okSeg {
			t.Fatalf("unexpected data type for segment-array %T", valueSegPair)
		}

		mSegPath := filepath.Join(valueSegPair[0].Mirror.DataDirectory, "postgresql.conf")
		host := valueSegPair[0].Mirror.Hostname
		coordinator := config.Get("coordinator")

		if value, ok = coordinator.(cli.Segment); !ok {
			t.Fatalf("unexpected data type for coordinator %T", value)
		}

		dirDeleted := make(chan bool)
		go func() {
			for {
				select {
				case <-dirDeleted:
					return
				default:
					cmdStr := fmt.Sprintf("if [ -f %s ]; then echo 'exists'; fi", mSegPath)
					cmd := exec.Command("ssh", host, cmdStr)
					output, err := cmd.Output()
					if err != nil {
						t.Errorf("unexpected error: %#v", err)
					}

					if strings.TrimSpace(string(output)) == "exists" {
						cmdStr := fmt.Sprintf("rm -rf %s", mSegPath)
						cmd := exec.Command("ssh", host, cmdStr)
						_, err := cmd.CombinedOutput()
						if err != nil {
							t.Errorf("unexpected error: %#v", err)
						}
						dirDeleted <- true
					}
					time.Sleep(1 * time.Second)
				}
			}
		}()

		result, err := testutils.RunInitCluster(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %s, %v", result.OutputMsg, err)
		}

		//Check if we got a message to rollback
		expectedOut := "[INFO]:-Please run gp init cluster --clean to rollback"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		result, err = testutils.RunInitCluster(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %s, %v", result.OutputMsg, err)
		}

		expectedOut = "[ERROR]:-Cluster creation failed, error gpinitsystem has failed previously. Run gp init cluster --clean before creating cluster again"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		initResult, err := testutils.RunInitCluster("--clean")
		if err != nil {
			t.Fatalf("Error while cleaning cluster: %#v", err)
		}

		clusterExpectedOut := "[INFO]:-clean cluster command successful"
		if !strings.Contains(initResult.OutputMsg, clusterExpectedOut) {
			t.Fatalf("got %q, want %q", initResult.OutputMsg, clusterExpectedOut)
		}

	})

	t.Run("cluster creation fails, user inputs no and changes are not rolled back", func(t *testing.T) {
		var (
			ok           bool
			valueSegPair []cli.SegmentPair
			value        cli.Segment
			okSeg        bool
		)

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		primarySegs := config.Get("segment-array")
		if valueSegPair, okSeg = primarySegs.([]cli.SegmentPair); !okSeg {
			t.Fatalf("unexpected data type for segment-array %T", valueSegPair)
		}

		mSegPath := filepath.Join(valueSegPair[0].Mirror.DataDirectory, "postgresql.conf")
		host := valueSegPair[0].Mirror.Hostname
		coordinator := config.Get("coordinator")

		if value, ok = coordinator.(cli.Segment); !ok {
			t.Fatalf("unexpected data type for coordinator %T", value)
		}

		dirDeleted := make(chan bool)
		go func() {
			for {
				select {
				case <-dirDeleted:
					return
				default:
					cmdStr := fmt.Sprintf("if [ -f %s ]; then echo 'exists'; fi", mSegPath)
					cmd := exec.Command("ssh", host, cmdStr)
					output, err := cmd.Output()
					if err != nil {
						t.Errorf("unexpected error: %#v", err)
					}

					if strings.TrimSpace(string(output)) == "exists" {
						cmdStr := fmt.Sprintf("rm -rf %s", mSegPath)
						cmd := exec.Command("ssh", host, cmdStr)
						_, err := cmd.CombinedOutput()
						if err != nil {
							t.Errorf("unexpected error: %#v", err)
						}
						dirDeleted <- true
					}
					time.Sleep(1 * time.Second)
				}
			}
		}()

		result, err := testutils.RunInitClusterwithUserInput("n", configFile)

		if err != nil {
			t.Fatalf("unexpected error: %s, %v", result.OutputMsg, err)
		}

		//Check if we got a message to rollback
		expectedOut := "[INFO]:-Please run gp init cluster --clean to rollback"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		//Check the postgres processes are not running
		var hostlist []string
		hostlist = append(hostlist, value.Hostname)
		result, _ = testutils.CheckifClusterisRunning(hostlist)
		if result.ExitCode == 0 && strings.TrimSpace(result.OutputMsg) == "2" {
			t.Errorf("psotgres processes should be running")
		}

		//Check if the contents of data directory are not removed
		hostDataDirMap := make(map[string][]string)
		hostDataDirMap[value.Hostname] = append(hostDataDirMap[value.Hostname], value.DataDirectory)
		result, _ = testutils.CheckifdataDirIsEmpty(hostDataDirMap)
		if result.ExitCode == 0 && strings.TrimSpace(result.OutputMsg) == "1" {
			t.Fatalf("data directory %s should not be empty", value.DataDirectory)
		}

		initResult, err := testutils.RunInitCluster("--clean")
		if err != nil {
			t.Fatalf("Error while cleaning cluster: %#v", err)
		}

		clusterExpectedOut := "[INFO]:-clean cluster command successful"
		if !strings.Contains(initResult.OutputMsg, clusterExpectedOut) {
			t.Fatalf("got %q, want %q", initResult.OutputMsg, clusterExpectedOut)
		}

	})

	t.Run("cluster creation fails, user inputs y and rollback succeeds", func(t *testing.T) {
		var ok bool

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		mirrorSegs := config.Get("segment-array")
		valueSegPair, ok := mirrorSegs.([]cli.SegmentPair)
		if !ok {
			t.Fatalf("unexpected data type for segment-array %T", mirrorSegs)
		}

		MirrorHostName := valueSegPair[0].Mirror.Hostname
		cmdStr := fmt.Sprintf("mkdir -p %s && chmod 700 %s && touch %s/abc.txt", valueSegPair[0].Mirror.DataDirectory, valueSegPair[0].Mirror.DataDirectory, valueSegPair[0].Mirror.DataDirectory)

		cmdObj := exec.Command("ssh", MirrorHostName, cmdStr)
		_, errSeg := cmdObj.Output()
		if errSeg != nil {
			t.Fatalf("unexpected error : %v", errSeg)
		}

		result, err := testutils.RunInitClusterwithUserInput("y", configFile)
		if err != nil {
			t.Fatalf("unexpected error: %s, %v", result.OutputMsg, err)
		}

		//Check if we got a message to rollback
		expectedOut := "[INFO]:-clean cluster command successful"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		coordinator := config.Get("coordinator")
		value, ok := coordinator.(cli.Segment)
		if !ok {
			t.Fatalf("unexpected data type for coordinator %T", value)
		}

		var hostlist []string
		hostlist = append(hostlist, value.Hostname)
		for _, seg := range valueSegPair {
			hostlist = append(hostlist, seg.Primary.Hostname)
		}

		//Verify there are no postgres processes running in co-ordinator and primary
		result, err = testutils.CheckifClusterisRunning(hostlist)
		if result.ExitCode != 0 && strings.TrimSpace(result.OutputMsg) != "2" {
			t.Errorf("postgres processes should not be running err: %v", err)
		}

		hostDataDirMap := make(map[string][]string)
		hostDataDirMap[value.Hostname] = append(hostDataDirMap[value.Hostname], value.DataDirectory)
		for _, seg := range valueSegPair {
			host := seg.Primary.Hostname
			dataDir := seg.Primary.DataDirectory
			hostDataDirMap[host] = append(hostDataDirMap[host], dataDir)

		}

		//Check if the contents of the directories are removed.
		result, _ = testutils.CheckifdataDirIsEmpty(hostDataDirMap)
		if result.ExitCode != 0 && strings.TrimSpace(result.OutputMsg) != "1" {
			t.Fatalf("data directory should be empty")
		}

	})
}
