import assert from "node:assert/strict";
import test from "node:test";

import { createCDPClient } from "./document-metadata-api-shape-v2-cdp.mjs";

const terminalPattern = /^HARNESS FAILURE: Chrome DevTools connection terminated$/;

test("pending call rejects when the socket closes", async () => {
    const socket = new FakeSocket();
    const client = createCDPClient(socket);
    const pending = client.call("Page.navigate");

    socket.dispatch("close");

    await assert.rejects(withTimeout(pending), { message: terminalPattern });
});

test("all pending calls reject with one error when the socket errors", async () => {
    const socket = new FakeSocket();
    const client = createCDPClient(socket);
    const calls = [
        client.call("Runtime.enable"),
        client.call("Page.enable"),
        client.call("Runtime.evaluate"),
    ];

    socket.dispatch("error", { error: new Error("socket failed") });

    const failures = await Promise.all(calls.map((call) => rejection(withTimeout(call))));
    assert.equal(failures.length, 3);
    assert.ok(failures.every((failure) => failure === failures[0]));
    assert.match(failures[0].message, terminalPattern);
});

test("error followed by close settles each call once and ignores late messages", async () => {
    const socket = new FakeSocket();
    const client = createCDPClient(socket);
    let settlements = 0;
    const pending = client.call("Runtime.evaluate").catch((error) => {
        settlements++;
        throw error;
    });

    socket.dispatch("error", { error: new Error("socket failed") });
    socket.dispatch("close");
    socket.dispatch("message", {
        data: JSON.stringify({ id: 1, result: { result: { value: true } } }),
    });

    await assert.rejects(withTimeout(pending), { message: terminalPattern });
    assert.equal(settlements, 1);
    client.close();
    client.close();
    assert.ok(socket.closeCalls <= 1);

    const openSocket = new FakeSocket();
    const openClient = createCDPClient(openSocket);
    openClient.close();
    openClient.close();
    assert.equal(openSocket.closeCalls, 1);
});

test("call after closure rejects immediately without sending", async () => {
    const socket = new FakeSocket();
    const client = createCDPClient(socket);
    socket.dispatch("close");

    await assert.rejects(withTimeout(client.call("Page.enable")), {
        message: terminalPattern,
    });
    assert.equal(socket.sent.length, 0);
});

test("send failure closes the client and rejects future calls", async () => {
    const socket = new FakeSocket();
    socket.sendError = new Error("send failed");
    const client = createCDPClient(socket);

    const firstFailure = await rejection(client.call("Page.enable"));
    const secondFailure = await rejection(client.call("Runtime.enable"));

    assert.equal(firstFailure, secondFailure);
    assert.match(firstFailure.message, /^HARNESS FAILURE: Chrome DevTools command send failed: send failed$/);
    assert.equal(socket.sendCalls, 1);
});

test("successful response preserves normal CDP behavior", async () => {
    const socket = new FakeSocket();
    const client = createCDPClient(socket);
    const pending = client.call("Runtime.evaluate", { expression: "1 + 1" });
    const request = JSON.parse(socket.sent[0]);

    socket.dispatch("message", {
        data: JSON.stringify({ id: request.id, result: { result: { value: 2 } } }),
    });

    assert.deepEqual(await pending, { result: { value: 2 } });
});

function withTimeout(promise, milliseconds = 50) {
    return Promise.race([
        promise,
        new Promise((_, reject) => {
            setTimeout(() => {
                reject(new Error("TEST TIMEOUT: CDP promise did not settle"));
            }, milliseconds);
        }),
    ]);
}

async function rejection(promise) {
    try {
        await promise;
    } catch (error) {
        return error;
    }
    assert.fail("promise resolved, want rejection");
}

class FakeSocket {
    constructor() {
        this.listeners = new Map();
        this.sent = [];
        this.sendCalls = 0;
        this.closeCalls = 0;
        this.sendError = null;
    }

    addEventListener(type, listener, options = {}) {
        const listeners = this.listeners.get(type) ?? [];
        listeners.push({ listener, once: Boolean(options.once) });
        this.listeners.set(type, listeners);
    }

    send(value) {
        this.sendCalls++;
        if (this.sendError) throw this.sendError;
        this.sent.push(value);
    }

    close() {
        this.closeCalls++;
    }

    dispatch(type, event = {}) {
        const listeners = [...(this.listeners.get(type) ?? [])];
        for (const entry of listeners) {
            entry.listener(event);
            if (entry.once) {
                const current = this.listeners.get(type) ?? [];
                this.listeners.set(type, current.filter((candidate) => candidate !== entry));
            }
        }
    }
}
