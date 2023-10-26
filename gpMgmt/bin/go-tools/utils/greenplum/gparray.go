package greenplum

import (
	"fmt"
	"github.com/greenplum-db/gp-common-go-libs/dbconn"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Segment struct {
	dbid          int
	contentId     int
	role          string
	prefRole      string
	mode          string
	state         string
	port          int
	dataDirectory string
	hostAddress   string
	hostName      string
}

//func main() {
//
//	primarys := []Segment{
//		{
//			port:          8002,
//			dataDirectory: "/Users/ravoorsh/workspace/gpdb/gpAux/gpdemo/datadirs/dbfast8/demoDataDir8",
//			hostName:      "ravoorsh3MD6R.vmware.com",
//			hostAddress:   "ravoorsh3MD6R.vmware.com",
//		},
//		{
//			port:          8003,
//			dataDirectory: "/Users/ravoorsh/workspace/gpdb/gpAux/gpdemo/datadirs/dbfast9/demoDataDir9",
//			hostName:      "ravoorsh3MD6R.vmware.com",
//			hostAddress:   "ravoorsh3MD6R.vmware.com",
//		},
//		{
//			port:          8004,
//			dataDirectory: "/Users/ravoorsh/workspace/gpdb/gpAux/gpdemo/datadirs/dbfast10/demoDataDir10",
//			hostName:      "ravoorsh3MD6R.vmware.com",
//			hostAddress:   "ravoorsh3MD6R.vmware.com",
//		},
//	}
//
//	err = nil
//	err = Segment.RegisterSegments(primarys)
//	if err != nil {
//		fmt.Println("Unable to register primaries to Segment")
//	}
//
//	GPArray, err = Segment.ReadGpSegmentConfig()
//	if err != nil {
//		fmt.Println("Unable to get data from gp_segment_configuration")
//	}
//
//	fmt.Println(len(GPArray))
//	fmt.Println(GPArray)
//
//	gpPrimary, err := GetPrimarySegments(GPArray)
//	if err != nil {
//		fmt.Println("Unable to get data from gp_segment")
//	}
//
//	fmt.Println(gpPrimary)
//}

func (seg *Segment) ReadGpSegmentConfig() ([]Segment, error) {
	// Returns the contents of gp_segment_configuration table
	conn := dbconn.NewDBConnFromEnvironment("postgres")
	defer conn.Close()

	conerr := conn.Connect(1, true)
	if conerr != nil {
		fmt.Println("Connection failed")
		return nil, conerr
	}

	query := "select dbid, content, role, preferred_role, mode, status, port, datadir, hostname, address " +
		"from pg_catalog.gp_segment_configuration  order by content asc, role desc;"
	rows, err := conn.Query(query)

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return seg.BuildGpArray(rows)
}

func (seg *Segment) BuildGpArray(rows *sqlx.Rows) ([]Segment, error) {

	result := []Segment{}

	for rows.Next() {
		dest := Segment{}

		if rErr := rows.Scan(
			&dest.dbid,
			&dest.contentId,
			&dest.role,
			&dest.prefRole,
			&dest.mode,
			&dest.state,
			&dest.port,
			&dest.dataDirectory,
			&dest.hostName,
			&dest.hostAddress,
		); rErr != nil {
			return nil, rErr
		}

		result = append(result, dest)
	}
	return result, nil
}

func (seg *Segment) GetPrimarySegments(gpArray []Segment) ([]Segment, error) {

	var result []Segment

	for item := range gpArray {
		if seg.isSegmentPrimary(gpArray[item]) {
			result = append(result, gpArray[item])
		}
	}
	return result, nil
}

func (seg *Segment) isSegmentPrimary(item Segment) bool {
	role := item.role
	prefRole := item.prefRole
	if item.contentId >= 0 && ((role == ROLE_PRIMARY) || (prefRole == ROLE_PRIMARY)) {
		return true
	}
	return false
}

func (seg *Segment) RegisterSegment(segs []Segment) error {

	conn := dbconn.NewDBConnFromEnvironment("postgres")
	defer conn.Close()

	conerr := conn.Connect(1, true)
	if conerr != nil {
		fmt.Println("Connection failed")
		return nil
	}

	addSegmentQueryString := "SELECT pg_catalog.gp_add_segment_primary( '%s', '%s', %d, '%s');"
	for _, segment := range segs {
		addSegmentQuery := fmt.Sprintf(addSegmentQueryString, segment.hostName, segment.hostAddress, segment.port, segment.dataDirectory)
		fmt.Println(addSegmentQuery)

		_, err := conn.Exec(addSegmentQuery)
		if err != nil {
			return err
		}
	}
	return nil
}
