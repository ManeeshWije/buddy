# buddy

buddy is an AI assistant that lives in your terminal, ready to answer or provide guidance when it comes to Linux commands, the terminal, etc. It utilizes a Lambda function that talks to AWS Bedrock running Llama3.3-70B. The api is then exposed using API Gateway and secured using an API key along with rate limiting in place. In terms of storage, the application uses DynamoDB to store users and their chat history which is used as context for when invoking the model multiple times.

### Building

The `Makefile` provides multiple targets depending on what you want to build. Usually, you can just use `make` which will build both the Lambda (cmd/buddy/main.go) and the cli (cmd/cli/cli.go).

If you make any changes to Terraform or the Lambda code, you can run `make deploy` which will build the code, zip the excutable, and apply the Terraform file. Note that it zips an executable called `bootstrap` as that's what Amazon's `provider.al2` requires for the Lambda function.

### Running

Make sure you have provisioned your AWS using the `main.tf` file, it will create all the services needed to run the CLI, things like:

- Lambda function
- IAM roles/policies
- DynamoDB tables
- API Gateway methods, usage plans, and api keys
- and more

Make sure the API_URL and API_KEY variables are present in your environment via `export API_URL= and export API_KEY=`. The API_URL will be your API gateway invoke url in the form of `https://<your-url>.execute-api.us-east-2.amazonaws.com/prod/chat`

You can then run the CLI using `make cli`

### Demo

![](https://github.com/ManeeshWije/buddy/blob/main/buddy.GIF)
