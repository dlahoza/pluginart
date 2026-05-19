import socket
import unittest

from pluginart.protocol import MAX_FRAME_SIZE, recv_frame, send_frame


class ProtocolTests(unittest.TestCase):
    def test_frame_round_trip(self):
        a, b = socket.socketpair()
        try:
            send_frame(a, 3, b"hello")
            self.assertEqual(recv_frame(b), (3, b"hello"))
        finally:
            a.close()
            b.close()

    def test_max_frame_rejected(self):
        a, b = socket.socketpair()
        try:
            with self.assertRaises(Exception):
                send_frame(a, 3, b"x" * (MAX_FRAME_SIZE + 1))
        finally:
            a.close()
            b.close()
