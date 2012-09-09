# udp tests
import socket
import binascii
import array

UDP_IP = "127.0.0.1"
UDP_PORT = 50002

class LBNS():

    def __init__(self):
        self.sock = socket.socket(socket.AF_INET, # Internet
                                  socket.SOCK_DGRAM) # UDP
        self.sock.bind(self.sock.getsockname()) # we listen for responses too
        print "sock=", self.sock.getsockname()

    def lookup(self, service):
            message = "%c%s" % (len(service), service)
            self.sock.sendto(message, (UDP_IP, UDP_PORT))
            (buf, addr) = self.sock.recvfrom(1024)

            buf = array.array('B',buf)  # we actually want an array of bytes here

            print addr, "=>", buf

            if len(buf) != 10:
                print "unexpected data length: ", len(buf)

            expected_hash = binascii.crc32(service) & 0xffffffff
            got_hash = (buf[3] << 24 | buf[2] << 16 | buf[1] << 8 | buf[0])
            if got_hash == 0:
                print "unknown service '%s'" % service
            elif expected_hash != got_hash:
                print "wrong packet response recieved (out-of-order?)"
                print "expected=%08x" % expected_hash
                print "got=%08x" % got_hash
            else:
                print "addr: %d.%d.%d.%d:%d" % (buf[4], buf[5], buf[6], buf[7], buf[8] << 8 | buf[9])


if __name__ == '__main__':
    l = LBNS()
    print "service1"
    l.lookup("service1")

    print "service2"
    l.lookup("service2")

    # force an out-of-order message
    print "sending request for service1 to force out-of-order response"
    message = "\x08service1"
    l.sock.sendto(message, (UDP_IP, UDP_PORT))

    print "service2 (again)"
    l.lookup("service2")
