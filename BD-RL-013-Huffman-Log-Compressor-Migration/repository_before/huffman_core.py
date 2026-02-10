# filename: huffman_core.py

import heapq
from collections import Counter

class HuffmanNode:
 def __init__(self, char, freq):
    self.char = char
    self.freq = freq
    self.left = None
    self.right = None

 def __lt__(self, other):
    return self.freq < other.freq

class HuffmanLogic:
 def build_tree(self, data):
    # Frequency analysis of the input byte data
    freqs = Counter(data)
    # Build a priority queue for leaf nodes
    priority_queue = [HuffmanNode(char, freq) for char, freq in freqs.items()]
    heapq.heapify(priority_queue)

    # Iteratively merge nodes to form the binary tree
    while len(priority_queue) > 1:
        left = heapq.heappop(priority_queue)
        right = heapq.heappop(priority_queue)
        merged = HuffmanNode(None, left.freq + right.freq)
        merged.left = left
        merged.right = right
        heapq.heappush(priority_queue, merged)

    return priority_queue[0] if priority_queue else None

 def generate_codes(self, node, current_code="", codes={}):
    if node is None:
        return
    if node.char is not None:
        codes[node.char] = current_code
        self.generate_codes(node.left, current_code + "0", codes)
        self.generate_codes(node.right, current_code + "1", codes)
        return codes
