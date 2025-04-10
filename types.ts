export interface Request {
    userID: string;
    query: string;
    conversationID?: string;
}

export interface Response {
    conversationID: string;
    response: string;
}

export interface ChatMessage {
    role: string;
    content: string;
}

export interface LlamaRequest {
    prompt: string;
    temperature: number;
}

export interface LlamaResponse {
    generation: string;
}

export interface Message {
    conversationID: string;
    messageID: string;
    userID: string;
    role: string;
    content: string;
    timestamp: string;
}
