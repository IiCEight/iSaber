// Local UI review fixture only. No database, credentials, or persistent writes.
// The application uses it only when BACKEND_ORIGIN is explicitly overridden.
// This is not the Go API; publishing/authentication are deliberately unsupported.
// Never use this fixture as a production service.
import { createServer } from 'node:http';
const category = { id: '10000000-0000-4000-8000-000000000001', name: 'Technology', slug: 'technology' };
const tag = { id: '10000000-0000-4000-8000-000000000002', name: 'Go', slug: 'go' };
const markdown = `A blog is a small system with a long memory. These representative passages test the reading experience with actual Markdown structure, rather than a screenshot of placeholder paragraphs.

## A place for long-form reading

The page should keep its rhythm as the article grows. A reader can arrive from the archive, follow a section link, select a passage, and continue on a phone without losing their place.

> The most useful interface makes room for the work it contains.

### 中文与 English 混合排版

把日常写下来，也是一种探索。技术文章不只有代码，还有思考的过程、问题的背景，以及尚未完成的想法。读者应该能够自然地阅读标点、链接和段落，而不被动画打断。

Read the [article collection](/archive), or visit [About](/about).

## Code and data

\`\`\`go
package main

import "fmt"

func main() {
    fmt.Println("A small blog, a long journey — 写下今天的想法")
    // A deliberately long line checks horizontal keyboard scrolling without widening the reading column or pushing the navigation off the screen.
}
\`\`\`

| Stage | Reader outcome | English and Chinese | A longer description |
| --- | --- | --- | --- |
| Write | Keep a thought | 写下想法 | Markdown stays readable in Typora and on the web. |
| Publish | Share a thread | 发布文章 | The owner explicitly chooses what becomes public. |

## Math without breaking the page

Inline math $E = mc^2$ belongs inside the paragraph.

$$
\\sum_{n=1}^{\\infty} \\frac{1}{n^2} = \\frac{\\pi^2}{6}, \\qquad f(x) = \\int_{-\\infty}^{\\infty} e^{-t^2}\\cos(xt)\\,dt
$$

## Images and references

![Geometric study in blue and coral](/api/v1/articles/11111111-1111-4111-8111-111111111111/assets/22222222-2222-4222-8222-222222222222)

- Notes should remain easy to scan.
- Links should remain easy to focus.
- Long addresses such as https://example.invalid/a/very/long/path/that/should/wrap/without/making/the/entire/document/scroll/horizontally are deliberately included.

${Array.from({length:6},(_,i)=>`## ${i+1}. A longer thread — 继续探索\n\n${'A clear reading column lets a thought develop over several paragraphs. 文字与代码，都需要恰当的空间。 '.repeat(4)}\n`).join('\n')}
`;
const articles = Array.from({length:7},(_,i)=>({id: i ? `11111111-1111-4111-8111-${String(i+1).padStart(12,'0')}` : '11111111-1111-4111-8111-111111111111',slug: i===0 ? 'reading-study' : i===1 ? 'chinese-study' : `field-note-${i}`,title: i===0 ? 'Building a quieter corner of the internet' : i===1 ? '把日常写下来，也是一种探索：关于文字、代码与仍在生长的想法' : i===2 ? 'UnderstandingConcurrentSystemsWithoutLosingTheThreadOrTheReaderAcrossScreenSizes' : `Field notes ${i}: things learned along the way`,summary: i===1 ? '记录探索，写下思考。让每一个想法，都有继续生长的空间。' : 'Notes on writing, software, and the small decisions that make a personal website worth returning to.',language:i===1?'zh':'en',written_at:'2026-10-24',published_at:'2026-11-08T08:30:00Z',updated_at:'2026-11-11T07:00:00Z',category:i===3?null:category,tags:i===3?[]:[tag],markdown:i===3?'A short note without headings.':markdown,comments_enabled:true}));
let mode='normal';
function json(res,status,data){res.writeHead(status,{'Content-Type':'application/json','Cache-Control':'no-store'});res.end(JSON.stringify(data));}
const server=createServer((req,res)=>{
 const u=new URL(req.url,'http://fixture.invalid');
 if(u.pathname==='/__review/mode' && req.method==='POST'){mode=u.searchParams.get('value');return json(res,200,{mode});}
 if(req.method!=='GET')return json(res,405,{code:'review_read_only'});
 if(mode==='outage')return json(res,503,{code:'unavailable'});
 if(u.pathname==='/api/v1/categories')return json(res,200,{items:[category],next_cursor:null});
 if(u.pathname==='/api/v1/tags')return json(res,200,{items:[tag],next_cursor:null});
 if(u.pathname.endsWith('/assets/22222222-2222-4222-8222-222222222222')){res.writeHead(200,{'Content-Type':'image/svg+xml'});return res.end('<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="600" viewBox="0 0 1200 600"><rect width="1200" height="600" fill="#176fe4"/><circle cx="500" cy="300" r="240" fill="#f16b3b"/><circle cx="760" cy="300" r="240" fill="#f65cad" fill-opacity=".8"/></svg>');}
 if(u.pathname.endsWith('/comments'))return json(res,200,{items:[{id:'sample-comment',display_name:'Review reader',body:'A useful thread. 继续写下去！',created_at:'2026-11-10T12:00:00Z'}],next_cursor:null});
 if(['/api/v1/articles','/api/v1/articles/search'].includes(u.pathname)){
  let items=mode==='empty'?[]:articles;
  for(const k of ['category','tag','language']){const v=u.searchParams.get(k);if(v)items=items.filter(a=>k==='language'?a.language===v:k==='category'?a.category?.slug===v:a.tags.some(t=>t.slug===v));}
  if(u.searchParams.get('q'))items=items.filter(a=>(a.title+' '+a.summary+' '+a.markdown).toLowerCase().includes(u.searchParams.get('q').toLowerCase()));
  const limit=Math.min(6,Number(u.searchParams.get('limit')||6)),offset=u.searchParams.get('cursor')==='review-next'?6:0;
  return json(res,200,{items:items.slice(offset,offset+limit).map(({markdown,comments_enabled,...a})=>a),next_cursor:items.length>offset+limit?'review-next':null});
 }
 const article=articles.find(a=>u.pathname===`/api/v1/articles/${a.slug}`);
 return article?json(res,200,article):json(res,404,{code:'not_found'});
});
server.listen(18080,'127.0.0.1',()=>console.log('Read-only representative content fixture: http://127.0.0.1:18080'));
