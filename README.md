# buddy

buddy is an AI assistant that lives in your terminal, ready to answer or provide guidance when it comes to Linux commands, the terminal, etc. It utilizes a Lambda function that talks to AWS Bedrock running Llama3.3-70B. The api is then exposed using API Gateway and secured using an API key along with rate limiting in place. In terms of storage, the application uses DynamoDB to store users and their chat history which is used as context for when invoking the model multiple times.

### Building

The `Makefile` provides multiple targets depending on what you want to build.

- `make deploy` will first transpile the TypeScript files, then zip the code into `lambda.zip`, then deploy everything using `terraform apply`. You should use this when making any changes to `index.ts` or `main.tf`
- `make run` will run the cli via `npm run cli.js`
- You can also target individual stages via `make build-lambda, make build, etc`
- `make clean` will remove transpiled js files and the lambda zip

### Running

Make sure you have provisioned your AWS using the `main.tf` file, it will create all the services needed to run the CLI, things like:

- Lambda function
- IAM roles/policies
- DynamoDB tables
- API Gateway methods, usage plans, and api keys
- and more

Make sure the API_URL and API_KEY variables are present in your environment via a `.env` file. The API_URL will be your API gateway invoke url in the form of `https://<your-url>.execute-api.us-east-2.amazonaws.com/prod/chat`

You can then run the CLI using `make run`

### Demo

![](https://github.com/ManeeshWije/buddy/blob/main/buddy.gif)

### TODO

- Easier way to switch out models
- Provide more functionality apart from chatbot
- Stream responses for REAL (not simulated)
