
package scrapers

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/UTDNebula/api-tools/utils"
    "github.com/chromedp/cdproto/cdp"
    "github.com/chromedp/chromedp"
)

// ProgramStruct to hold the scraped data
type StudyAbroadProgram struct {
    Name     string `json:"name"`
    Location string `json:"location"`
    Term     string `json:"term"`
    Link     string `json:"link"`
}

func ScrapeStudyAbroad(outDir string) {
    // 1. Init chromedp (Reuse existing existing utils)
    ctx, cancel := utils.InitChromeDp()
    defer cancel()

    // 2. Define the Search URL
    // We use the direct URL with the query parameters to skip manually clicking "Search" buttons.
    // This URL includes the filter for "Faculty Led" programs
    targetURL := `https://utdallas-ea.terradotta.com/_portal/program-discovery?search=[{"filterType":"programDiscoveryProgramParameters","filterValues":[{"id":10004,"value":"Faculty%20Led"}]}]`

    log.Printf("Navigating to Study Abroad Portal...")

    // 3. Navigate and Wait for Results
    // We wait for the specific card container to ensure JS has finished loading data.
    var nodes []*cdp.Node
    err := chromedp.Run(ctx,
        chromedp.Navigate(targetURL),
        // Wait for the "grid" or "list" of programs to appear. 
        // Note: class names are guesses based on standard Terradotta layouts; 
        // Inspect the page (Right Click -> Inspect) to confirm the exact class name if this times out.
        chromedp.WaitVisible(`div[class*="program-card"]`, chromedp.ByQuery), 
        // Scroll to bottom to ensure all "lazy loaded" images/data are rendered
        chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil),
        chromedp.Sleep(2*time.Second), // Give it a moment to settle
        // Select all program cards
        chromedp.Nodes(`div[class*="program-card"]`, &nodes, chromedp.ByQueryAll),
    )
    if err != nil {
        log.Panicf("Failed to load search results: %v", err)
    }

    log.Printf("Found %d programs. Extracting details...", len(nodes))

    // 4. Extract Information from Each Card
    var programs []StudyAbroadProgram

    for _, node := range nodes {
        var name, location, term, link string
        
        // Run a sub-task for each node to extract text from its children
        // We use the node.FullXPath() to scope the search to *just* this card
        err := chromedp.Run(ctx,
            chromedp.Text(`h3, h4, .program-name`, &name, chromedp.ByQuery, chromedp.FromNode(node)),
            chromedp.Text(`span[class*="location"], .program-location`, &location, chromedp.ByQuery, chromedp.FromNode(node)),
            chromedp.Text(`span[class*="term"], .program-term`, &term, chromedp.ByQuery, chromedp.FromNode(node)),
            // Get the link from the anchor tag
            chromedp.AttributeValue(`a`, "href", &link, nil, chromedp.ByQuery, chromedp.FromNode(node)),
        )
        
        if err != nil {
            log.Printf("Warning: Failed to extract one card: %v", err)
            continue
        }

        // Clean up the link (it might be relative)
        if link != "" && link[0] == '/' {
            link = "https://utdallas-ea.terradotta.com" + link
        }

        programs = append(programs, StudyAbroadProgram{
            Name:     name,
            Location: location,
            Term:     term,
            Link:     link,
        })
    }

    // 5. Save Results to JSON
    // Save as structured data
    saveStudyAbroadData(programs, outDir)
}

func saveStudyAbroadData(programs []StudyAbroadProgram, outDir string) {
    if err := os.MkdirAll(outDir, 0777); err != nil {
        panic(err)
    }

    filePath := fmt.Sprintf("%s/study_abroad_faculty_led.json", outDir)
    file, err := os.Create(filePath)
    if err != nil {
        panic(err)
    }
    defer file.Close()

    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "  ")
    if err := encoder.Encode(programs); err != nil {
        panic(err)
    }

    log.Printf("Successfully saved %d programs to %s", len(programs), filePath)
}