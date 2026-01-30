import unittest
import sys
import os
import argparse
from unittest.mock import MagicMock, patch
import importlib.util

# Load the service module
from importlib.machinery import SourceFileLoader
service = SourceFileLoader("service", os.path.abspath("service")).load_module()

class TestService(unittest.TestCase):
    def setUp(self):
        self.args = argparse.Namespace(
            bind=MagicMock(),
            hostname="testhost",
            default_subject="Test Message",
            telegram_token="TOKEN",
            telegram_chat="CHAT",
            state_dir=MagicMock(),
            socket_timeout=10.0,
            listen_timeout=10.0,
            max_payload_size=1024
        )

    @patch('service.urlopen')
    def test_send_payload_text(self, mock_urlopen):
        # Mock successful response
        mock_response = MagicMock()
        mock_response.status = 200
        mock_urlopen.return_value = mock_response

        data = b"Subject: Hello\n\nThis is a test message."
        timestamp = "1234567890"

        service.send_payload(data, timestamp, self.args)

        # Check if urlopen was called with the correct URL for sendMessage
        args, kwargs = mock_urlopen.call_args
        url = args[0]
        self.assertIn("sendMessage", url)
        # Note: The message is URL encoded, so we check for parts
        # The subject extracted from data is "Hello", not "Test Message"
        self.assertIn("Hello", url)

    @patch('service.urlopen')
    def test_send_payload_default_subject(self, mock_urlopen):
        # Mock successful response
        mock_response = MagicMock()
        mock_response.status = 200
        mock_urlopen.return_value = mock_response

        data = b"Body only\nThis is a test message."
        timestamp = "1234567890"

        service.send_payload(data, timestamp, self.args)

        # Check if urlopen was called with the correct URL for sendMessage
        args, kwargs = mock_urlopen.call_args
        url = args[0]
        self.assertIn("sendMessage", url)
        self.assertIn("Test%20Message", url) # Encoded default subject

    @patch('service.urlopen')
    def test_send_payload_file(self, mock_urlopen):
        # Force file send by using a long message
        long_message = "A" * (service.MESSAGE_LENGTH_LIMIT_BEFORE_FILE + 10)
        data = f"Subject: Long\n\n{long_message}".encode('utf-8')
        timestamp = "1234567890"

        mock_response = MagicMock()
        mock_response.status = 200
        mock_urlopen.return_value = mock_response

        service.send_payload(data, timestamp, self.args)

        # Check if urlopen was called with a Request object (for sendDocument)
        args, kwargs = mock_urlopen.call_args
        request = args[0]
        self.assertTrue(isinstance(request, service.Request))
        self.assertIn("sendDocument", request.full_url)

if __name__ == '__main__':
    unittest.main()
