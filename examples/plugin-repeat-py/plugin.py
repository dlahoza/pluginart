# Plugin entrypoint. Edit this file as needed.
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent / 'plugin'))

from pluginart import serve
from contract import CONTRACT_HASH
from handler import handle

if __name__ == '__main__':
    serve(handle, contract_hash=CONTRACT_HASH)
