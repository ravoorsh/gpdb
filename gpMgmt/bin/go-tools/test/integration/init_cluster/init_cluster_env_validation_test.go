package init_cluster

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/greenplum-db/gpdb/gp/cli"
	"github.com/greenplum-db/gpdb/gp/test/integration/testutils"
)

func TestEnvValidation(t *testing.T) {
	t.Run("when the given data directory is not empty", func(t *testing.T) {
		var value cli.Segment
		var ok bool

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		coordinator := config.Get("coordinator")
		if value, ok = coordinator.(cli.Segment); !ok {
			t.Fatalf("unexpected data type for coordinator %T", coordinator)
		}

		err = os.MkdirAll(value.DataDirectory, 0777)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		_, err = os.Create(filepath.Join(value.DataDirectory, "abc.txt"))
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		defer os.RemoveAll(value.DataDirectory)

		result, err := testutils.RunInitCluster(configFile)
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			t.Fatalf("got %v, want exit status 1", err)
		}

		expectedOut := fmt.Sprintf("[ERROR]:-validating hosts: host: %s, directory not empty:[%s]\n", value.Hostname, value.DataDirectory)
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}
	})

	t.Run("when the given port is already in use", func(t *testing.T) {
		var value cli.Segment
		var ok bool

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		coordinator := config.Get("coordinator")
		if value, ok = coordinator.(cli.Segment); !ok {
			t.Fatalf("unexpected data type for coordinator %T", value)
		}

		lis, err := net.Listen("tcp", net.JoinHostPort(value.Address, strconv.Itoa(value.Port)))
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}
		defer lis.Close()

		result, err := testutils.RunInitCluster(configFile)
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			t.Fatalf("got %v, want exit status 1", err)
		}

		expectedOut := fmt.Sprintf("ERROR]:-validating hosts: host: %s, ports already in use: [%d], check if cluster already running", value.Hostname, value.Port)
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}
	})

	t.Run("when the initdb does not have appropriate permission", func(t *testing.T) {
		var value cli.Segment
		var ok bool

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		coordinator := config.Get("coordinator")
		if value, ok = coordinator.(cli.Segment); !ok {
			t.Fatalf("unexpected data type for coordinator %T", value)
		}

		gpHome := os.Getenv("GPHOME")
		if gpHome == "" {
			t.Fatal("GPHOME environment variable not set")
		}

		initdbFilePath := filepath.Join(gpHome, "bin", "initdb")

		if err := os.Chmod(initdbFilePath, 0444); err != nil {
			t.Fatalf("unexpected error during changing initdb file permission: %#v", err)
		}
		defer func() {
			if err := os.Chmod(initdbFilePath, 0755); err != nil {
				t.Fatalf("unexpected error during changing initdb file permission: %#v", err)
			}
		}()

		result, err := testutils.RunInitCluster(configFile)
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			t.Fatalf("got %v, want exit status 1", err)
		}

		expectedOut := fmt.Sprintf("[ERROR]:-validating hosts: host: %s, file %s does not have execute permissions", value.Hostname, initdbFilePath)

		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}
	})

	t.Run("when data directory is not empty and --force is given for gp init command", func(t *testing.T) {
		var value cli.Segment
		var ok bool

		configFile := testutils.GetTempFile(t, "config.json")
		config := GetDefaultConfig(t)

		err := config.WriteConfigAs(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		coordinator := config.Get("coordinator")
		if value, ok = coordinator.(cli.Segment); !ok {
			t.Fatalf("unexpected data type for coordinator %T", value)
		}

		err = os.MkdirAll(value.DataDirectory, 0777)
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		_, err = os.Create(filepath.Join(value.DataDirectory, "abc.txt"))
		if err != nil {
			t.Fatalf("unexpected error: %#v", err)
		}

		result, err := testutils.RunInitCluster("--force", configFile)

		if err != nil {
			t.Fatalf("Error while intializing cluster: %#v", err)
		}
		expectedOut := "[INFO]:-Cluster initialized successfully"
		if !strings.Contains(result.OutputMsg, expectedOut) {
			t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
		}

		//FIXME - with force flag is triggered immediately again initilization fails, below part of code should be enabled once bug is fixed
		// initResult, err := testutils.RunInitCluster("--force", configFile)

		// if err != nil {
		// 	t.Errorf("Error while intializing cluster: %#v", err)
		// }

		// clusterExpectedOut := "[INFO]:-Cluster initialized successfully"
		// if !strings.Contains(result.OutputMsg, expectedOut) {
		// 	t.Errorf("got %q, want %q", initResult.OutputMsg, clusterExpectedOut)
		// }

		_, err = testutils.DeleteCluster()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	//FIXME: Validation needs to be added for when mirror port is in use
	// t.Run("when the port is already in use for mirror segment", func(t *testing.T) {
	// 	var ok bool

	// 	configFile := testutils.GetTempFile(t, "config.json")
	// 	config := GetDefaultConfig(t)

	// 	err := config.WriteConfigAs(configFile)
	// 	if err != nil {
	// 		t.Fatalf("unexpected error: %#v", err)
	// 	}

	// 	primarySegs := config.Get("segment-array")
	// 	valueSegPair, ok := primarySegs.([]cli.SegmentPair)

	// 	if !ok {
	// 		t.Fatalf("unexpected data type for segment-array %T", primarySegs)
	// 	}

	// 	MirrorHostName := valueSegPair[0].Mirror.Hostname
	// 	MirrorPort := valueSegPair[0].Mirror.Port
	// 	MirrorAddress := valueSegPair[0].Mirror.Address

	// 	cmd := exec.Command("ssh", MirrorHostName, "nc", "-l", MirrorAddress, strconv.Itoa(MirrorPort))
	// 	cmd_err := cmd.Start()
	// 	if cmd_err != nil {
	// 		t.Fatalf("failed to start listening on mirror host: %v", err)
	// 	}

	// 	result, err := testutils.RunInitCluster(configFile)
	// 	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
	// 		t.Fatalf("got %v, want exit status 1", err)
	// 	}

	// 	expectedOut := fmt.Sprintf("[ERROR]:-validating hosts: host: %s, ports already in use: [%d], check if cluster already running", valueSegPair[0].Primary.Hostname, valueSegPair[0].Primary.Port)
	// 	if !strings.Contains(result.OutputMsg, expectedOut) {
	// 		t.Fatalf("got %q, want %q", result.OutputMsg, expectedOut)
	// 	}

	// })

}
