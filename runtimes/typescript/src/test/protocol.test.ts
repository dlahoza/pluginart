import test from 'node:test';
import assert from 'node:assert/strict';
import { MAX_FRAME_SIZE, FrameReader, sendFrame } from '../protocol';

test('frame round trip', () => {
  const reader = new FrameReader();
  let written = Buffer.alloc(0);
  const fakeSocket = {
    write(chunk: Buffer) {
      written = Buffer.concat([written, chunk]);
      return true;
    },
  };

  sendFrame(fakeSocket as any, 3, Buffer.from('hello'));
  const next = reader.next();
  reader.feed(written);
  return next.then((frame) => {
    assert.equal(frame.type, 3);
    assert.equal(frame.payload.toString(), 'hello');
  });
});

test('max frame rejected', () => {
  const sock = { write() { return true; } };
  assert.throws(() => sendFrame(sock as any, 3, Buffer.alloc(MAX_FRAME_SIZE + 1)));
});
