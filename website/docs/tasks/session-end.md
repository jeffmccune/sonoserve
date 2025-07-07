# Session End Process

This document describes the steps to end a coding session and prepare for a new one. These instructions are for Claude Code to follow when ending a session.

## Steps to End Session

1. **Rename current prompts file**
   - Rename `website/docs/prompts.md` to `website/docs/session-X.md` (where X is the next session number)
   - Example: `prompts.md` → `session-4.md`

2. **Create new prompts file**
   - Create new `website/docs/prompts.md` with an h1 header named "Session Y - Prompts and Responses" (where Y is the next session number after X)
   - Example: If current session is 4, new session would be 5

3. **Add template content to new prompts.md**
   ```markdown
   # Session Y - Prompts and Responses

   ## Turn 1 - Summary here

   > My prompt here.

   Your response here.
   ```

4. **Format turns correctly**
   - Each prompt and response should be in an h2 section named like "Turn Z - Summary of turn here" where Z is the turn number for the session (starting at 1)
   - User prompts should be in blockquotes using `> My prompt here.`
   - Assistant responses should be plain text: `Your response here.`

5. **Update the completed session file**
   - Update the `session-X.md` file with the last prompt and response of the session
   - Do NOT add this final session-end content to the new `prompts.md` file

6. **Commit the changes**
   - Add both files to git
   - Commit with a message like "End Session X and prepare Session Y"

## Example Workflow

If ending session 4 and preparing for session 5:

1. `mv website/docs/prompts.md website/docs/session-4.md`
2. Create new `website/docs/prompts.md` with "Session 5 - Prompts and Responses"
3. Add the session-end turn to `session-4.md`
4. Commit: "End Session 4 and prepare Session 5"