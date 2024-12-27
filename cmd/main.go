package main
import (
    "flag"
    "log"
    "os"
    "strings"
    "github.com/3-shake/alert-menta/internal/ai"
    "github.com/3-shake/alert-menta/internal/github"
    "github.com/3-shake/alert-menta/internal/utils"
)
func main() {
    // Get command line arguments
    var (
        repo        = flag.String("repo", "", "Repository name")
        owner       = flag.String("owner", "", "Repository owner")
        issueNumber = flag.Int("issue", 0, "Issue number")
        intent      = flag.String("intent", "", "Question or intent for the 'ask' command")
        command     = flag.String("command", "", "Commands to be executed by AI.Commands defined in the configuration file are available.")
        configFile  = flag.String("config", "", "Configuration file")
        gh_token    = flag.String("github-token", "", "GitHub token")
        oai_key     = flag.String("api-key", "", "OpenAI api key")
    )
    flag.Parse()
    if *repo == "" || *owner == "" || *issueNumber == 0 || *gh_token == "" || *command == "" || *configFile == "" {
        flag.PrintDefaults()
        os.Exit(1)
    }
    // Initialize a logger
    logger := log.New(
        os.Stdout, "[alert-menta main] ",
        log.Ldate|log.Ltime|log.Llongfile|log.Lmsgprefix,
    )
    // Load configuration
    cfg, err := utils.NewConfig(*configFile)
    if err != nil {
        logger.Fatalf("Error loading configuration: %s", err)
    }
    // Validate command
    if _, ok := cfg.Ai.Commands[*command]; !ok {
        allowedCommands := make([]string, 0, len(cfg.Ai.Commands))
        for cmd := range cfg.Ai.Commands {
            allowedCommands = append(allowedCommands, cmd)
        }
        logger.Fatalf("Invalid command: %s. Allowed commands are %s.", *command, strings.Join(allowedCommands, ", "))
    }
    // Create a GitHub Issues instance. From now on, you can control GitHub from this instance.
    issue := github.NewIssue(*owner, *repo, *issueNumber, *gh_token)
    if issue == nil {
        logger.Fatalf("Failed to create GitHub issue instance")
    }
    // Get Issue's information(e.g. Title, Body) and add them to the user prompt except for comments by Actions.
    title, err := issue.GetTitle()
    if err != nil {
        logger.Fatalf("Error getting Title: %v", err)
    }
    body, err := issue.GetBody()
    if err != nil {
        logger.Fatalf("Error getting Body: %v", err)
    }
    if cfg.System.Debug.Log_level == "debug" {
        logger.Println("Title:", *title)
        logger.Println("Body:", *body)
    }
    user_prompt := "Title: " + *title + "\nBody: " + *body + "\n"
    // Get comments under the Issue and add them to the user prompt except for comments by Actions.
    comments, err := issue.GetComments()
    if err != nil {
        logger.Fatalf("Error getting comments: %v", err)
    }
    for _, v := range comments {
        if *v.User.Login == "github-actions[bot]" {
            continue
        }
        if cfg.System.Debug.Log_level == "debug" {
            logger.Printf("%s: %s", *v.User.Login, *v.Body)
        }
        user_prompt += *v.User.Login + ": " + *v.Body + "\n"
    }
    // Set system prompt
    var system_prompt string
    if *command == "ask" {
        if *intent == "" {
            logger.Fatalf("Error: intent is required for 'ask' command")
        }
        system_prompt = cfg.Ai.Commands[*command].System_prompt + *intent + "\n"
    } else {
        system_prompt = cfg.Ai.Commands[*command].System_prompt
    }
    prompt := ai.Prompt{UserPrompt: user_prompt, SystemPrompt: system_prompt}
    logger.Println("\x1b[34mPrompt: |\n", prompt.SystemPrompt, prompt.UserPrompt, "\x1b[0m")
    // Get response from OpenAI or VertexAI
    var aic ai.Ai
    if cfg.Ai.Provider == "openai" {
        if *oai_key == "" {
14:15
logger.Fatalf("Error: Please provide your Open AI API key.")
        }
        aic = ai.NewOpenAIClient(*oai_key, cfg.Ai.OpenAI.Model)
        logger.Println("Using OpenAI API")
        logger.Println("OpenAI model:", cfg.Ai.OpenAI.Model)
    } else if cfg.Ai.Provider == "vertexai" {
        aic = ai.NewVertexAIClient(cfg.Ai.VertexAI.Project, cfg.Ai.VertexAI.Region, cfg.Ai.VertexAI.Model)
        logger.Println("Using VertexAI API")
        logger.Println("VertexAI model:", cfg.Ai.VertexAI.Model)
    } else {
        logger.Fatalf("Error: Invalid provider")
    }
    // Notify that AI is starting to respond
    if err := issue.NotifyAIStatus(true); err != nil {
        logger.Printf("Error notifying AI start status: %v", err)
    }
    // Get response from AI
    comment, err := aic.GetResponse(prompt)
    if err != nil {
        logger.Printf("Error getting AI response: %v", err)
        // Optionally notify AI completion here
    } else {
        logger.Println("Response:", comment)
        // Post a comment on the Issue
        err = issue.PostComment(comment)
        if err != nil {
            logger.Fatalf("Error creating comment: %s", err)
        }
    }
    // Notify that AI processing is complete
    if err := issue.NotifyAIStatus(false); err != nil {
        logger.Printf("Error notifying AI completion status: %v", err)
    }
}