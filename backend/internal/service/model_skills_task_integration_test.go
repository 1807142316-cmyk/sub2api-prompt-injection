//go:build unit

package service

import (
 "context"
 "encoding/json"
 "strings"
 "testing"
)

func TestModelSkillsTaskRoutingProtocols(t *testing.T) {
 skills:=[]ModelSkill{
  {ID:"base",Name:"base",Content:"BASELINE_EXECUTION_EVIDENCE"},
  {ID:"static",Name:"static",Content:`<!-- sub2api:skill-route:v1 {"id":"reverse-static","any_regex":["(?i)Ghidra|反汇编"],"not_regex":[]} -->`+"\nSTATIC_MODULE_BODY"},
  {ID:"debug",Name:"debug",Content:`<!-- sub2api:skill-route:v1 {"id":"reverse-debug","any_regex":["(?i)\\bgdb\\b"],"not_regex":[]} -->`+"\nDEBUG_MODULE_BODY"},
  {ID:"bad",Name:"bad",Content:`<!-- sub2api:skill-route:v1 {"id":"bad","any_regex":["["],"not_regex":[]} -->`+"\nBAD_MODULE_BODY"},
 }
 global,_:=json.Marshal(ModelSkillRuleConfig{DefaultSkillIDs:[]string{"base","static","debug","bad"}})
 rawSkills,_:=json.Marshal(skills)
 for _,protocol:=range []string{"responses","chat_completions","anthropic","gemini"} {
  for _,tc:=range []struct{task string;static,debug bool}{{"Use Ghidra to inspect the function",true,false},{"Explain the weather tomorrow",false,false},{"Set a gdb breakpoint",false,true}} {
   t.Run(protocol+"/"+tc.task,func(t *testing.T){
    repo:=&modelSkillsProbeSettingRepo{values:map[string]string{SettingKeyGlobalModelSkillRules:string(global),SettingKeyModelSkills:string(rawSkills)}}
    settings:=&SettingService{settingRepo:repo}
    root:=map[string]any{"model":"test-model"}
    switch protocol {
    case "responses":root["instructions"]="CLIENT_ORIGINAL_MARKER";root["input"]=tc.task
    case "chat_completions","anthropic":root["messages"]=[]any{map[string]any{"role":"user","content":tc.task}};if protocol=="anthropic" {root["system"]="CLIENT_ORIGINAL_MARKER"} else {root["messages"]=append([]any{map[string]any{"role":"developer","content":"CLIENT_ORIGINAL_MARKER gdb Ghidra"}},root["messages"].([]any)...)}
    case "gemini":root["systemInstruction"]=map[string]any{"parts":[]any{map[string]any{"text":"CLIENT_ORIGINAL_MARKER Ghidra gdb"}}};root["contents"]=[]any{map[string]any{"role":"user","parts":[]any{map[string]any{"text":tc.task}}}}
    }
    body,_:=json.Marshal(root)
    got,err:=settings.ApplyGroupDefaultSkills(context.Background(),&Group{DefaultSkillIDs:[]string{"static"}},protocol,"test-model",body)
    if err!=nil {t.Fatal(err)}
    text:=string(got)
    for token,want:=range map[string]int{"BASELINE_EXECUTION_EVIDENCE":1,"CLIENT_ORIGINAL_MARKER":1,"BAD_MODULE_BODY":0} {if n:=strings.Count(text,token);n!=want {t.Fatalf("%s count=%d want=%d",token,n,want)}}
    for token,want:=range map[string]bool{"STATIC_MODULE_BODY":tc.static,"DEBUG_MODULE_BODY":tc.debug} {n:=strings.Count(text,token);if (n==1)!=want||n>1 {t.Fatalf("%s count=%d selected=%v",token,n,want)}}
    if strings.Contains(text,"sub2api:skill-route:") {t.Fatal("route metadata leaked upstream")}
    again,err:=settings.ApplyGroupDefaultSkills(context.Background(),nil,protocol,"test-model",got);if err!=nil||string(again)!=text {t.Fatal("deduplication changed routed request")}
   })
  }
 }
}
