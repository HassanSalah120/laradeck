import test from 'node:test';
import assert from 'node:assert/strict';
import {parseRecordChanges} from '../src/recordEditing.ts';

test('preserves boolean, JSON and numeric types while omitting untouched fields',()=>{
 const before={id:1,active:true,meta:{safe:true},count:2,name:'same',display_label:'derived'};
 assert.deepEqual(parseRecordChanges(before,{...before,active:'false',meta:'{"safe":false}',count:'3'},{active:'boolean',meta:'array',count:'integer'}),{active:false,meta:{safe:false},count:3});
});
test('invalid values never produce a write payload',()=>{
 assert.throws(()=>parseRecordChanges({active:true},{active:'maybe'},{active:'boolean'}),/true or false/);
 assert.throws(()=>parseRecordChanges({meta:{}},{meta:'{broken'},{meta:'array'}),/valid JSON/);
 assert.throws(()=>parseRecordChanges({meta:{}},{meta:'"text"'},{meta:'array'}),/object or array/);
 assert.throws(()=>parseRecordChanges({count:1},{count:'Infinity'},{count:'integer'}),/valid number/);
 assert.throws(()=>parseRecordChanges({count:1},{count:'2.5'},{count:'integer'}),/safe integer/);
});
test('NULL is explicit and strings retain string semantics',()=>{
 assert.deepEqual(parseRecordChanges({text:'x',decimal:'1.23',active:null},{text:null,decimal:'2.45',active:'false'},{decimal:'decimal:2',active:'boolean'}),{text:null,decimal:'2.45',active:false});
 assert.deepEqual(parseRecordChanges({text:null},{text:''},{}),{text:''});
 assert.deepEqual(parseRecordChanges({text:'true'},{text:'false'},{}),{text:'false'});
});
test('does not resend unchanged accessors, timestamps or the primary key',()=>{
 const before={id:1,name:'name',meta:{safe:true},created_at:'yesterday',updated_at:'yesterday',deleted_at:null,creation_date:'2026-01-01',modified_date:'2026-01-02',display_label:'derived'};
 assert.deepEqual(parseRecordChanges(before,{...before,id:99,created_at:'today',updated_at:'today',deleted_at:'today',creation_date:'today',modified_date:'today',meta:'{"safe":true}'},{meta:'array'}),{});
 assert.deepEqual(parseRecordChanges(before,{...before},{}),{});
});

test('a float cast remains editable when its previous value is a whole number',()=>{
 assert.deepEqual(parseRecordChanges({amount:2},{amount:'2.5'},{amount:'float'}),{amount:2.5});
});
