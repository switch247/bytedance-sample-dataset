
# // filename: huffman_service.py

from huffman_core import HuffmanLogic


class HuffmanService:
    def __init__(self):
        self.logic = HuffmanLogic()

    def compress(self, data):
        if not data:
            return b""
        tree = self.logic.build_tree(data)
        codes = self.logic.generate_codes(tree)

        # Python string concatenation approach is the primary performance bottleneck
        encoded_str = "".join([codes[char] for char in data])

        # Calculate padding needed for byte alignment
        padding = 8 - (len(encoded_str) % 8)
        encoded_str += "0" * padding

        # Manual conversion of bit-string to bytearray
        b = bytearray()
        for i in range(0, len(encoded_str), 8):
            byte = encoded_str[i:i+8]
            b.append(int(byte, 2))
        return bytes(b)
