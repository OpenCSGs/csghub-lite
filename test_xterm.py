import asyncio
from playwright.async_api import async_playwright

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome")
        page = await browser.new_page()
        await page.set_viewport_size({"width": 1024, "height": 768})
        
        print("Navigating directly to shell...")
        await page.goto("http://localhost:11435/ai-apps/shell?app_id=claude-code&session_id=38d6b23a1d6b2fe126c72743")
        
        await asyncio.sleep(3)
        await page.screenshot(path="user_issue.png")
        
        # get innerHTML
        html = await page.content()
        with open("user_issue.html", "w") as f:
            f.write(html)
            
        print("Done")

if __name__ == "__main__":
    asyncio.run(main())