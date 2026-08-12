export function createCDPClient(socket) {
    let nextID = 1;
    const pending = new Map();
    let terminalError = null;

    const terminate = (error) => {
        if (terminalError) return terminalError;
        terminalError = error;
        const requests = [...pending.values()];
        pending.clear();
        for (const request of requests) request.reject(error);
        return error;
    };

    const terminateConnection = (cause) => {
        if (terminalError) return terminalError;
        return terminate(new Error(
            "HARNESS FAILURE: Chrome DevTools connection terminated",
            cause instanceof Error ? { cause } : undefined,
        ));
    };

    socket.addEventListener("close", () => {
        terminateConnection();
    });
    socket.addEventListener("error", (event) => {
        terminateConnection(event?.error);
    });

    socket.addEventListener("message", (event) => {
        if (terminalError) return;
        const message = JSON.parse(event.data);
        if (!message.id || !pending.has(message.id)) return;
        const request = pending.get(message.id);
        pending.delete(message.id);
        if (message.error) request.reject(new Error(message.error.message));
        else request.resolve(message.result);
    });

    return {
        call(method, params = {}) {
            if (terminalError) return Promise.reject(terminalError);
            return new Promise((resolveCall, reject) => {
                const id = nextID++;
                pending.set(id, { resolve: resolveCall, reject });
                try {
                    socket.send(JSON.stringify({ id, method, params }));
                } catch (cause) {
                    if (!terminalError) {
                        terminate(new Error(
                            `HARNESS FAILURE: Chrome DevTools command send failed: ${cause?.message ?? cause}`,
                            { cause },
                        ));
                    }
                }
            });
        },
        async evaluate(expression) {
            const result = await this.call("Runtime.evaluate", {
                expression,
                returnByValue: true,
                awaitPromise: true,
            });
            if (result.exceptionDetails) {
                throw new Error(
                    `APP FAILURE: browser evaluation failed: ${JSON.stringify(result.exceptionDetails)}`,
                );
            }
            return result.result.value;
        },
        close() {
            if (terminalError) return;
            terminateConnection();
            socket.close();
        },
    };
}
