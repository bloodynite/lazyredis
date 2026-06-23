package tui

import (
	"sort"
	"strings"
)

const folderSeparator = ":"

type treeNode struct {
	name     string
	fullPath string
	isFolder bool
	children treeNodes
	depth    int
}

type treeNodes []*treeNode

func buildKeyTree(keys []string, order sortOrder) []*treeNode {
	root := make(treeNodes, 0)
	folderMap := make(map[string]*treeNode)

	for _, key := range keys {
		parts := splitKey(key)
		if len(parts) <= 1 {
			root = append(root, &treeNode{
				name:     key,
				fullPath: key,
				isFolder: false,
				depth:    0,
			})
			continue
		}

		var parent *treeNode
		currentPath := ""
		for i, part := range parts[:len(parts)-1] {
			if i == 0 {
				currentPath = part
			} else {
				currentPath += folderSeparator + part
			}

			if existing, ok := folderMap[currentPath]; ok {
				parent = existing
			} else {
				folder := &treeNode{
					name:     part,
					fullPath: currentPath,
					isFolder: true,
					depth:    i,
					children: make(treeNodes, 0),
				}
				folderMap[currentPath] = folder
				if parent == nil {
					root = append(root, folder)
				} else {
					parent.children = append(parent.children, folder)
				}
				parent = folder
			}
		}

		leafName := parts[len(parts)-1]
		leafPath := key
		if parent != nil {
			leafPath = parent.fullPath + folderSeparator + leafName
		}
		parent.children = append(parent.children, &treeNode{
			name:     leafName,
			fullPath: leafPath,
			isFolder: false,
			depth:    len(parts) - 1,
		})
	}

	sortTreeNodes(root, order)
	return root
}

func sortTreeNodes(nodes treeNodes, order sortOrder) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].isFolder != nodes[j].isFolder {
			return nodes[i].isFolder
		}
		a, b := strings.ToLower(nodes[i].name), strings.ToLower(nodes[j].name)
		if order == sortZA {
			return a > b
		}
		return a < b
	})
	for _, n := range nodes {
		if n.isFolder && len(n.children) > 0 {
			sortTreeNodes(n.children, order)
		}
	}
}

func flattenTree(nodes []*treeNode, expanded map[string]bool, depth int) []*treeNode {
	var result []*treeNode
	for _, node := range nodes {
		node.depth = depth
		result = append(result, node)
		if node.isFolder {
			if expanded[node.fullPath] {
				children := flattenTree(node.children, expanded, depth+1)
				result = append(result, children...)
			}
		}
	}
	return result
}

func splitKey(key string) []string {
	if key == "" {
		return nil
	}
	return splitKeyRune(key, folderSeparator[0])
}

func splitKeyRune(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
