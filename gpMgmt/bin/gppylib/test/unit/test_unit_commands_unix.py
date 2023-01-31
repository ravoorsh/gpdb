import os
from unittest import mock

from mock import patch

from commands.unix import RemoveFile, RemoveDirectory, RemoveDirectoryContents, RemoveGlob, REMOTE, Command

from bin.gppylib.commands.unix import check_postgres_process_remote, kill_9_segment_processes
from .gp_unittest import *


class CommandsUnix(GpTestCase):
    """
    this is technically an integration test since it uses actual bash shell processes
    """

    def setUp(self):
        self.dir = "/tmp/command_unix_test"
        if not os.path.exists(self.dir):
            os.mkdir(self.dir)
        self.filepath = self.dir + "/foo"

        self.apply_patches([
            patch("uuid.uuid4", return_value="Patched"),
        ])
        open(self.filepath, 'a').close()

    def tearDown(self):
        RemoveDirectory.local("", self.dir)
        Command.propagate_env_map.clear()
        super(CommandsUnix, self).tearDown()


    def test_RemoveFile_for_file_succeeds_locally(self):
        RemoveFile.local("testing", self.filepath)
        self.assertFalse(os.path.exists(self.filepath))
        self.assertTrue(os.path.exists(self.dir))

    def test_RemoveFile_for_file_succeeds_with_environment(self):
        cmd = RemoveFile("testing", self.filepath)
        cmd.propagate_env_map['foo'] = 1
        cmd.run(validateAfter=True)
        self.assertFalse(os.path.exists(self.filepath))
        self.assertTrue(os.path.exists(self.dir))

    def test_RemoveFile_for_file_with_nonexistent_file_succeeds(self):
        RemoveFile.local("testing", "/doesnotexist")
        self.assertTrue(os.path.exists(self.filepath))
        self.assertTrue(os.path.exists(self.dir))


    def test_RemoveDirectory_succeeds_locally(self):
        RemoveDirectory.local("testing", self.dir)
        self.assertFalse(os.path.exists(self.dir))
        self.assertFalse(os.path.exists("/tmp/emptyForRemovePatched"))

    def test_RemoveDirectory_with_nonexistent_dir_succeeds(self):
        RemoveDirectory.local("testing", "/doesnotexist")
        self.assertTrue(os.path.exists(self.dir))

    def test_RemoveDirectory_succeeds_with_slash(self):
        RemoveDirectory.local("testing", self.dir + "/")
        self.assertFalse(os.path.exists(self.dir))
        self.assertFalse(os.path.exists("/tmp/emptyForRemovePatched"))

    def test_RemoveDirectoryContents_succeeds_locally(self):
        RemoveDirectoryContents.local("testing", self.dir)
        self.assertTrue(os.path.exists(self.dir))
        self.assertFalse(os.path.exists(self.filepath))
        self.assertFalse(os.path.exists("/tmp/emptyForRemovePatched"))

    def test_RemoveDirectoryContents_with_nonexistent_dir_succeeds(self):
        RemoveDirectoryContents.local("testing", "/doesnotexist")
        self.assertTrue(os.path.exists(self.dir))
        self.assertFalse(os.path.exists("/tmp/emptyForRemovePatched"))

    def test_RemoveDirectoryContents_with_slash(self):
        RemoveDirectoryContents.local("testing", self.dir + "/")
        self.assertTrue(os.path.exists(self.dir))
        self.assertFalse(os.path.exists(self.filepath))
        self.assertFalse(os.path.exists("/tmp/emptyForRemovePatched"))

    def test_RemoveGlob_succeeds_locally(self):
        RemoveGlob.local("testing", self.dir + "/f*")
        self.assertFalse(os.path.exists(self.filepath))
        self.assertTrue(os.path.exists(self.dir))

    def test_RemoveGlob_with_nonexistent_succeeds(self):
        RemoveGlob.local("testing", "/doesnotexist")
        self.assertTrue(os.path.exists(self.filepath))
        self.assertTrue(os.path.exists(self.dir))

    @mock.patch('gppylib.commands.base.Command.run', return_value=False)
    def test_check_postgres_process_remote_command_fails(self, mock1):
        expected = "Unable to get the pid list of processes for segment"
        actual = check_postgres_process_remote("/tmp", 1234, "localhost", "LOCAL")
        self.assertIn(expected, actual)

    @mock.patch('gppylib.commands.base.Command.run')
    @mock.patch('gppylib.commands.base.Command.get_results', read_data=0)
    def test_check_postgres_process_remote_succeeds(self, mock1, mock2):
        expected = True
        actual = check_postgres_process_remote("/tmp", 1234, "localhost", "LOCAL")
        self.assertEqual(expected, actual)

    @mock.patch('gppylib.commands.base.Command.run')
    @mock.patch('gppylib.commands.base.Command.get_results', read_data='1234,1235')
    def test_check_postgres_process_remote_fails(self, mock1, mock2):
        expected = None
        actual = check_postgres_process_remote("/tmp", 1234, "localhost", "LOCAL")
        self.assertEqual(expected, actual)

    @mock.patch('os.kill', return_value=0)
    def test_kill_9_segment_processes_kill_succeeds(self, mock1):
        expected = None
        actual = kill_9_segment_processes('/tmp', '123, 456, 789')
        self.assertEqual(expected, actual)

    @mock.patch('os.kill', return_value=1)
    def test_kill_9_segment_processes_kill_fails(self, mock1):
        expected = "Failed to kill processes for segment"
        actual = kill_9_segment_processes('/tmp', '123, 456, 789')
        self.assertIn(expected, actual)

    # Note: remote tests use ssh to localhost, which is not really something that should be done in a true unit test.
    # we leave them here, commented out, for development usage only.

    # def test_RemoveFile_for_file_succeeds_remotely(self):
    #     RemoveFile.remote("testing", "localhost", self.filepath)
    #     self.assertFalse(os.path.exists(self.filepath))
    #     self.assertTrue(os.path.exists(self.dir))

    # def test_RemoveFile_for_file_succeeds_remotely_with_environment(self):
    #     cmd = RemoveFile("testing", self.filepath, REMOTE, "localhost")
    #     cmd.propagate_env_map["foo"] = 1
    #     cmd.run(validateAfter=True)
    #     self.assertFalse(os.path.exists(self.filepath))
    #     self.assertTrue(os.path.exists(self.dir))
    #
    # def test_RemoveDirectory_succeeds_remotely(self):
    #     RemoveDirectory.remote("testing", "localhost", self.dir)
    #     self.assertFalse(os.path.exists(self.dir))
    #     self.assertFalse(os.path.exists("/tmp/emptyForRemovePatched"))

    # def test_RemoveDirectoryContents_succeeds_remotely(self):
    #     RemoveDirectoryContents.remote("testing", "localhost", self.dir)
    #     self.assertTrue(os.path.exists(self.dir))
    #     self.assertFalse(os.path.exists(self.filepath))
    #     self.assertFalse(os.path.exists("/tmp/emptyForRemovePatched"))

    # def test_RemoveGlob_succeeds_remotely(self):
    #     RemoveGlob.remote("testing", "localhost", self.dir + "/f*")
    #     self.assertFalse(os.path.exists(self.filepath))
    #     self.assertTrue(os.path.exists(self.dir))


if __name__ == '__main__':
    run_tests()
