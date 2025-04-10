import { APIGatewayProxyEvent, APIGatewayProxyResult } from "aws-lambda";
import {
    DynamoDBClient,
    QueryCommand,
    PutItemCommand,
} from "@aws-sdk/client-dynamodb";
import {
    BedrockRuntimeClient,
    InvokeModelCommand,
} from "@aws-sdk/client-bedrock-runtime";
import { marshall, unmarshall } from "@aws-sdk/util-dynamodb";
import {
    Request,
    Response,
    ChatMessage,
    LlamaRequest,
    LlamaResponse,
    Message,
} from "./types";
import * as dotenv from "dotenv";
dotenv.config();

const systemPrompt = `You are a helpful CLI assistant specialized in Linux and terminal commands. Provide concise, accurate information and examples when asked. Be concise and never output a large wall of text that is hard to parse through for the user. Only respond to what the user is specifically asking about. Never assume or make up information about the user's environment or previous conversations. Only use information that the user has explicitly provided. DO NOT include any <|system|>, <|user|>, or <|assistant|> tags in your responses.`;

const cleanResponse = (response: string): string => {
    return response
        .replace(/<\|system\|>[\s\S]*?(?=<\|user\||<\|assistant\||$)/gi, "")
        .replace(/<\|user\|>[\s\S]*?(?=<\|system\||<\|assistant\||$)/gi, "")
        .replace(/<\|assistant\|>/gi, "")
        .replace(/<\|(system|user|assistant)\|>/gi, "")
        .trim();
};

export const handler = async (
    event: APIGatewayProxyEvent,
): Promise<APIGatewayProxyResult> => {
    if (!event.body) {
        return { statusCode: 400, body: "Missing request body" };
    }

    let req: Request;
    try {
        req = JSON.parse(event.body);
    } catch (err) {
        return { statusCode: 400, body: `Invalid JSON: ${err}` };
    }

    const dynamo = new DynamoDBClient({ region: process.env.AWS_REGION });
    const bedrock = new BedrockRuntimeClient({
        region: process.env.AWS_REGION,
    });
    let conversationID = req.conversationID || `conv-${Date.now()}`;

    const systemMessage: ChatMessage = {
        role: "system",
        content: systemPrompt,
    };
    let messages: ChatMessage[] = [systemMessage];
    if (req.conversationID) {
        try {
            const history = await getConversationHistory(
                req.userID,
                conversationID,
                dynamo,
            );
            const recentMessages = history.slice(-4); // last 2 user-assistant pairs
            messages.push(...recentMessages, {
                role: "user",
                content: req.query,
            });
            //messages.push(...history);
        } catch (err) {
            return {
                statusCode: 500,
                body: `Error retrieving history: ${err}`,
            };
        }
    }

    messages.push({ role: "user", content: req.query });
    await storeMessage(req.userID, conversationID, "user", req.query, dynamo);

    const prompt = [
        `<|system|>\n${systemPrompt}`,
        ...messages.slice(1).map((m) => `<|${m.role}|>\n${m.content}`),
        "<|assistant|>",
    ].join("\n");

    const llamaReq: LlamaRequest = { prompt, temperature: 0.5 };
    const modelID =
        process.env.BEDROCK_MODEL_ID || "us.meta.llama3-3-70b-instruct-v1:0";

    let generation = "";
    try {
        const command = new InvokeModelCommand({
            modelId: modelID,
            contentType: "application/json",
            body: Buffer.from(JSON.stringify(llamaReq)),
        });

        const result = await bedrock.send(command);
        const parsed: LlamaResponse = JSON.parse(
            Buffer.from(result.body).toString(),
        );
        generation = cleanResponse(parsed.generation);
    } catch (err) {
        return { statusCode: 500, body: `Error invoking model: ${err}` };
    }

    await storeMessage(
        req.userID,
        conversationID,
        "assistant",
        generation,
        dynamo,
    );
    const response: Response = { conversationID, response: generation };

    return {
        statusCode: 200,
        headers: {
            "Content-Type": "application/json",
            "Access-Control-Allow-Origin": "*",
        },
        body: JSON.stringify(response),
    };
};

const getConversationHistory = async (
    userID: string,
    conversationID: string,
    client: DynamoDBClient,
): Promise<ChatMessage[]> => {
    const result = await client.send(
        new QueryCommand({
            TableName: process.env.MESSAGES_TABLE,
            KeyConditionExpression: "conversationID = :cid",
            ExpressionAttributeValues: marshall({
                ":cid": conversationID,
                ":uid": userID,
            }),
            FilterExpression: "userID = :uid",
            ScanIndexForward: true,
        }),
    );

    const items =
        result.Items?.map((item) => unmarshall(item) as Message) || [];
    return items
        .filter((m) => m.role === "user" || m.role === "assistant")
        .map((m) => ({ role: m.role, content: m.content }));
};

const storeMessage = async (
    userID: string,
    conversationID: string,
    role: string,
    content: string,
    client: DynamoDBClient,
): Promise<void> => {
    const message: Message = {
        conversationID,
        messageID: `msg-${Date.now()}`,
        userID,
        role,
        content,
        timestamp: new Date().toISOString(),
    };

    await client.send(
        new PutItemCommand({
            TableName: process.env.MESSAGES_TABLE,
            Item: marshall(message),
        }),
    );
};
