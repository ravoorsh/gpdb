@gpstop
Feature: gpstop behave tests

    @concourse_cluster
    @demo_cluster
    Scenario: gpstop succeeds
        Given the database is running
         When the user runs "gpstop -a"
         Then gpstop should return a return code of 0

    @concourse_cluster
    @demo_cluster
    Scenario: when there are user connections gpstop waits to shutdown until user switches to fast mode
        Given the database is running
          And the user asynchronously runs "psql postgres" and the process is saved
         When the user runs gpstop -a -t 4 --skipvalidation and selects f
          And gpstop should print "'\(s\)mart_mode', '\(f\)ast_mode', '\(i\)mmediate_mode'" to stdout
         Then gpstop should return a return code of 0

    @concourse_cluster
    @demo_cluster
    Scenario: when there are user connections gpstop waits to shutdown until user connections are disconnected
        Given the database is running
          And the user asynchronously runs "psql postgres" and the process is saved
          And the user asynchronously sets up to end that process in 6 seconds
         When the user runs gpstop -a -t 2 --skipvalidation and selects s
          And gpstop should print "There were 1 user connections at the start of the shutdown" to stdout
          And gpstop should print "'\(s\)mart_mode', '\(f\)ast_mode', '\(i\)mmediate_mode'" to stdout
         Then gpstop should return a return code of 0

    @demo_cluster
    Scenario: gpstop succeeds even if the standby host is unreachable
        Given the database is running
          And the catalog has a standby coordinator entry
         When the standby host is made unreachable
          And the user runs "gpstop -a"
         Then gpstop should print "Standby is unreachable, skipping shutdown on standby" to stdout
          And gpstop should return a return code of 0
          And the standby host is made reachable


    @concourse_cluster
    @demo_cluster
    Scenario: when gpstop -M smart fails abruptly all the postgres processes are killed
        Given the database is running
          And the user asynchronously runs "psql postgres" and the process is saved
          And the user asynchronously sets up to end that process in 6 seconds
         When the user runs gpst  op -a -t 2 --skipvalidation and selects s
          And gpstop should print "There were 1 user connections at the start of the shutdown" to stdout
          And gpstop should print "'\(s\)mart_mode', '\(f\)ast_mode', '\(i\)mmediate_mode'" to stdout
         Then gpstop should return a return code of 0

    @ravoorsh
    @concourse_cluster
    @demo_cluster
    Scenario: gpstop -M immediate kills all postgres and its child processes
         Given the database is running
         And all the segments are running
         When the user puts to sleep the check_pointRF process on the primary
         When the user runs /usr/local/gpdb/bin/gpstop -M immediate --skipvalidation and selects y
         Then gpstop should return a return code of 0
         Then all postgres processes are killed on all host

    @ravoorsh
    @concourse_cluster
    @demo_cluster
    Scenario: gpstop -M fast kills all postgres and its child processes
         Given the database is running
         And all the segments are running
         When the user puts to sleep the check_point process on the primary
         When the user runs /usr/local/gpdb/bin/gpstop -M fast --skipvalidation and selects y
         Then gpstop should return a return code of 0
         Then all postgres processes are killed on all host



