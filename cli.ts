import { Response, Request } from "./types";
import readline from "readline";
import "dotenv/config";

const apiUrl = process.env.API_URL;
const apiKey = process.env.API_KEY;
const userID = process.env.USER_ID || "default-user";
let conversationID = process.env.CONVERSATION_ID || "";

if (!apiUrl || !apiKey) {
    throw new Error(
        "ERROR: API_URL or API_KEY is not defined in the environment",
    );
}

const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
});

console.log("CLI Assistant is ready. Type your queries (type 'exit' to quit):");
console.log("------------------------------------------------------------");

async function askQuery(): Promise<void> {
    rl.question("> ", async (query) => {
        if (query.trim().toLowerCase() === "exit") {
            console.log("Goodbye!");
            rl.close();
            return;
        }

        const requestBody: Request = {
            userID,
            query,
            conversationID: conversationID,
        };

        try {
            showThinkingAnimation();
            const response = await fetch(apiUrl!, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "x-api-key": apiKey!,
                },
                body: JSON.stringify(requestBody),
            });

            stopThinkingAnimation();

            const responseText = await response.text();

            if (!response.ok) {
                console.error(
                    `API returned error (status ${response.status}):`,
                    responseText,
                );
                return askQuery();
            }

            const data: Response = JSON.parse(responseText);
            conversationID = data.conversationID;

            console.log(
                "------------------------------------------------------------",
            );
            console.log(data.response);
            console.log(
                "------------------------------------------------------------",
            );
        } catch (err) {
            stopThinkingAnimation();
            console.error("Error calling API:", err);
        }

        askQuery();
    });
}

// Thinking animation
let spinnerInterval: ReturnType<typeof setInterval>;

function showThinkingAnimation() {
    const spinners = ["-", "\\", "|", "/"];
    let i = 0;
    spinnerInterval = setInterval(() => {
        process.stdout.write(`\rThinking ${spinners[i++]}`);
        i %= spinners.length;
    }, 100);
}

function stopThinkingAnimation() {
    clearInterval(spinnerInterval);
    process.stdout.write("\r                    \r");
}

askQuery();
