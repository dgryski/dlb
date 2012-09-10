# udp tests
import socket
import binascii
import struct
import array
import random

UDP_IP = "127.0.0.1"
UDP_PORT = 50002

class LBNS():

    def __init__(self):
        self.sock = socket.socket(socket.AF_INET, # Internet
                                  socket.SOCK_DGRAM) # UDP
        self.sock.bind(self.sock.getsockname()) # we listen for responses too
        print "sock=", self.sock.getsockname()
        self.req = random.randint(0, 1 << 16)

    def lookup(self, service):
            self.req += 1
            message = struct.pack("!H%dp" % (len(service)+1), self.req % (1<<16), service)
            self.sock.sendto(message, (UDP_IP, UDP_PORT))
            (buf, addr) = self.sock.recvfrom(1024)

            buf = array.array('B',buf)  # we actually want an array of bytes here

            if len(buf) != 12:
                print "unexpected data length: ", len(buf)

            (seq, got_hash, packed_ip, port) = struct.unpack("!HIIH", buf)

            if seq != self.req:
                print "out of order packet:", seq, "!=", self.req

            expected_hash = binascii.crc32(service) & 0xffffffff
            if expected_hash != got_hash:
                print "bad service hash: expected=%08x got=%08x" % (expected_hash, got_hash)
            elif packed_ip == 0 and port == 0:
                print "=> unknown service"
            else:
                ip = socket.inet_ntop(socket.AF_INET, struct.pack('!I', packed_ip))
                print "=> addr: ", ip, ":", port

if __name__ == '__main__':
    l = LBNS()
    print "service1"
    l.lookup("service1")

    print "service2"
    l.lookup("service2")

    # force an out-of-order message
    print "sending request for service1 to force out-of-order response"
    message = "\xde\xad\x08service1"
    l.sock.sendto(message, (UDP_IP, UDP_PORT))

    print "service2 (again)"
    l.lookup("service2")
