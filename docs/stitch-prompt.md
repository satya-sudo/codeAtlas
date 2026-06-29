# Stitch Prompt For CodeAtlas

Design a modern product interface for **CodeAtlas**, a GitHub intelligence platform for engineering teams.

The product is not a generic dashboard and it is not an AI agent product. It is an internal-facing engineering intelligence tool that helps teams understand repository ownership, expertise, hotspots, coupling, and knowledge distribution across a codebase.

## Product context

CodeAtlas connects to GitHub repositories and answers questions like:

- who owns a module
- who is the best reviewer for a module
- which files change most often
- which files have the highest churn
- which files frequently change together
- what is the bus factor of a module
- how knowledge is distributed across the codebase

The platform architecture behind the product includes:

- GitHub OAuth for user login
- a GitHub App for repository installation and access
- repository onboarding and sync
- analytics computation
- knowledge graph generation

This first design should focus on the **setup and onboarding stage**, while still previewing the product the user will get after setup completes.

## Current implemented flow

This flow already exists in the product and the UI should be designed around it:

1. User lands on CodeAtlas
2. User signs in with GitHub
3. Frontend receives a JWT from the auth flow
4. User clicks **Install GitHub App**
5. Frontend requests the GitHub App install URL from the backend
6. User goes to GitHub and installs the app
7. GitHub redirects back with an installation ID
8. Frontend claims that installation on behalf of the authenticated user
9. The product confirms that the GitHub App installation is now linked to the current CodeAtlas account
10. The next future step is syncing repositories from that installation

The interface should make this flow feel clear, orderly, and trustworthy.

## Design goals

The UI should feel:

- modern
- clean
- product-focused
- structured
- calm
- technically credible

The UI should **not** feel:

- cluttered
- overdesigned
- like a Dribbble dashboard concept
- salesy
- “AI-generated”
- overloaded with decorative charts or unnecessary chrome

## Visual direction

Use a visual direction closer to:

- Linear
- GitHub
- Vercel
- a well-designed internal engineering product

Preferred traits:

- light theme
- strong typography
- restrained accent color
- clear layout hierarchy
- low-noise surfaces
- simple cards only where they help
- clean spacing and alignment

Avoid:

- giant gradients
- glassmorphism
- glowing blobs
- too many panels
- too many colors
- complex sidebars for this stage
- fake futuristic “AI analytics” styling

## What to design

Design a **single clean onboarding-oriented product page** for CodeAtlas that includes the following sections.

### 1. Top navigation

Keep it simple.

Include:

- CodeAtlas brand
- small product descriptor
- authenticated user state
- sign out action

No large marketing navigation.

### 2. Hero / onboarding header

This should be the most important part of the page.

It should communicate:

- what CodeAtlas is
- what the current stage of setup is
- what the user needs to do next

The hero should include:

- one strong headline
- one short explanatory paragraph
- primary action: **Sign in with GitHub**
- secondary action: **Install GitHub App**

The content should feel concise and product-oriented, not promotional.

### 3. Setup status

Add a clear status section that shows the current state of:

- authentication
- GitHub App installation
- installation claim / linking
- next backend step

This should feel like product state, not like a debug panel.

Use plain language such as:

- Not signed in
- Session active
- Returned from GitHub
- Claiming installation
- Installation linked
- Ready for repository sync

### 4. Product flow section

Add a simple visual or structured step list for the current flow:

1. Sign in
2. Install GitHub App
3. Return from GitHub
4. Claim installation
5. Sync repositories

The user should immediately understand:

- where they are now
- what has already happened
- what comes next

### 5. Installation detail panel

Add a lightweight detail area that can show:

- installation ID
- setup action
- claim status
- linked user

This should feel polished and readable, not raw JSON and not developer tooling UI.

### 6. Future product preview

Below onboarding, show a restrained preview of what CodeAtlas will eventually provide after setup.

Include simple believable examples of:

- repository overview metrics
- hotspot files
- ownership / reviewer insights

This section should support confidence in the product, but should not dominate the page.

It should feel like:

- “this is what you unlock after setup”

not:

- “this dashboard already does everything”

## Content guidance

Use copy that sounds like a real engineering product.

Tone should be:

- clear
- direct
- technically literate
- confident
- not overexplained

Avoid:

- hype
- abstract AI language
- marketing fluff
- vague platform claims

## UX priorities

Prioritize these in order:

1. clarity of setup flow
2. simplicity
3. credibility
4. ease of scanning
5. product preview

The setup flow should clearly be the main story.

## Layout guidance

Preferred layout:

- top nav
- hero
- compact status row
- two-column content sections below
- restrained preview panels

The page should feel balanced and spacious, not empty and not busy.

## States to support

The design should visually support these states:

- signed out
- signed in
- install not started
- returned from GitHub
- claim in progress
- claim successful
- error state if installation claim fails

Make these states feel intentional and well-designed.

## Deliverable expectation

Generate a clean, polished onboarding product page for CodeAtlas that feels like the first real screen of an engineering intelligence platform.

The page should successfully combine:

- GitHub sign in
- GitHub App installation
- installation claim state
- progress visibility
- a preview of the repository analytics product that comes next
