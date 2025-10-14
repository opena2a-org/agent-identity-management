"use client";

import { useState } from "react";
import { X, Loader2, CheckCircle, AlertCircle } from "lucide-react";
import { toast } from "sonner";
import { TagCategory } from "@/lib/api";

interface CreateTagModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: (tag: {
    key: string;
    value: string;
    category: TagCategory;
    description?: string;
    color?: string;
  }) => void;
}

interface FormData {
  key: string;
  value: string;
  category: TagCategory;
  description: string;
  color: string;
}

const TAG_CATEGORIES: { value: TagCategory; label: string }[] = [
  { value: "environment", label: "Environment" },
  { value: "data_classification", label: "Data Classification" },
  { value: "custom", label: "Custom" },
];

const PRESET_COLORS = [
  { hex: "#10B981", name: "Green" },
  { hex: "#3B82F6", name: "Blue" },
  { hex: "#F59E0B", name: "Amber" },
  { hex: "#EF4444", name: "Red" },
  { hex: "#8B5CF6", name: "Purple" },
  { hex: "#EC4899", name: "Pink" },
  { hex: "#06B6D4", name: "Cyan" },
  { hex: "#DC2626", name: "Dark Red" },
  { hex: "#6B7280", name: "Gray" },
];

export function CreateTagModal({
  isOpen,
  onClose,
  onSuccess,
}: CreateTagModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const [formData, setFormData] = useState<FormData>({
    key: "",
    value: "",
    category: "custom",
    description: "",
    color: "#3B82F6", // Default blue
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.key.trim()) {
      newErrors.key = "Tag key is required";
    } else if (!/^[a-z0-9_]+$/.test(formData.key)) {
      newErrors.key =
        "Tag key must be lowercase alphanumeric with underscores only";
    }

    if (!formData.value.trim()) {
      newErrors.value = "Tag value is required";
    } else if (!/^[a-z0-9_-]+$/.test(formData.value)) {
      newErrors.value =
        "Tag value must be lowercase alphanumeric with underscores/hyphens only";
    }

    if (!formData.category) {
      newErrors.category = "Please select a category";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const tagData = {
        key: formData.key.toLowerCase().trim(),
        value: formData.value.toLowerCase().trim(),
        category: formData.category,
        description: formData.description.trim() || undefined,
        color: formData.color,
      };

      // Call the onSuccess callback with the tag data
      onSuccess?.(tagData);
      setSuccess(true);

      // Show success toast
      toast.success("Tag Created Successfully", {
        description: `Custom tag "${tagData.key}:${tagData.value}" has been created and is now available.`,
      });

      // Close modal after a brief delay to show success message
      setTimeout(() => {
        handleClose();
      }, 1000);
    } catch (err) {
      console.error("Failed to create tag:", err);

      // Extract error message from different possible error formats
      let errorMessage = "Failed to create tag";

      if (err instanceof Error) {
        errorMessage = err.message;
      } else if (typeof err === "string") {
        errorMessage = err;
      } else if (err && typeof err === "object" && "message" in err) {
        errorMessage = (err as any).message;
      }

      setError(errorMessage);

      // Show error toast
      toast.error("Tag Creation Failed", {
        description: errorMessage,
        action: {
          label: "Retry",
          onClick: () => handleSubmit(new Event("submit") as any),
        },
      });
    } finally {
      setLoading(false);
    }
  };

  const resetForm = () => {
    setFormData({
      key: "",
      value: "",
      category: "custom",
      description: "",
      color: "#3B82F6",
    });
    setErrors({});
    setError(null);
    setSuccess(false);
  };

  const handleClose = () => {
    if (!loading) {
      resetForm();
      onClose();
    }
  };

  // Check if form has been modified
  const isFormDirty = () => {
    if (success) return false;
    return (
      formData.key.trim() !== "" ||
      formData.value.trim() !== "" ||
      formData.description.trim() !== "" ||
      formData.color !== "#3B82F6"
    );
  };

  // Handle click on overlay (outside modal)
  const handleOverlayClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      if (isFormDirty()) {
        if (
          confirm(
            "You have unsaved changes. Are you sure you want to close without saving?"
          )
        ) {
          handleClose();
        }
      } else {
        handleClose();
      }
    }
  };

  const getTagPreview = () => {
    if (!formData.key || !formData.value) return null;
    return `${formData.key}:${formData.value}`;
  };

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
      onClick={handleOverlayClick}
    >
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            Create Custom Tag
          </h2>
          <button
            onClick={handleClose}
            disabled={loading}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors disabled:opacity-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-6">
          {/* Success Message */}
          {success && (
            <div className="mb-6 p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg">
              <div className="flex items-start gap-3">
                <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400 flex-shrink-0 mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-green-800 dark:text-green-300">
                    Tag Created Successfully
                  </p>
                  <p className="text-xs text-green-700 dark:text-green-400 mt-1">
                    Your custom tag "{getTagPreview()}" has been created and is
                    now available.
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* Form */}
          {!success && (
            <form onSubmit={handleSubmit} className="space-y-4">
              {/* Error Message */}
              {error && (
                <div className="p-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg flex items-center gap-3">
                  <AlertCircle className="h-5 w-5 text-yellow-600 dark:text-yellow-400" />
                  <div className="flex-1">
                    <p className="text-sm text-yellow-800 dark:text-yellow-300">
                      {error}
                    </p>
                  </div>
                </div>
              )}

              {/* Tag Preview */}
              {getTagPreview() && (
                <div className="p-3 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
                  <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">
                    Preview:
                  </p>
                  <div
                    className="inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium text-white"
                    style={{ backgroundColor: formData.color }}
                  >
                    {getTagPreview()}
                  </div>
                </div>
              )}

              {/* Tag Key */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Tag Key <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={formData.key}
                  onChange={(e) =>
                    setFormData({ ...formData, key: e.target.value })
                  }
                  placeholder="e.g., team, region, project"
                  className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                    errors.key
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                  }`}
                  disabled={loading}
                />
                {errors.key && (
                  <p className="mt-1 text-xs text-red-500">{errors.key}</p>
                )}
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  The category or type of tag (lowercase, alphanumeric,
                  underscores only)
                </p>
              </div>

              {/* Tag Value */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Tag Value <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={formData.value}
                  onChange={(e) =>
                    setFormData({ ...formData, value: e.target.value })
                  }
                  placeholder="e.g., backend, us-east, customer-portal"
                  className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                    errors.value
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                  }`}
                  disabled={loading}
                />
                {errors.value && (
                  <p className="mt-1 text-xs text-red-500">{errors.value}</p>
                )}
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  The specific value for this tag (lowercase, alphanumeric,
                  underscores/hyphens)
                </p>
              </div>

              {/* Category */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Category <span className="text-red-500">*</span>
                </label>
                <select
                  value={formData.category}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      category: e.target.value as TagCategory,
                    })
                  }
                  className={`w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 ${
                    errors.category
                      ? "border-red-500"
                      : "border-gray-200 dark:border-gray-700"
                  }`}
                  disabled={loading}
                >
                  {TAG_CATEGORIES.map((cat) => (
                    <option key={cat.value} value={cat.value}>
                      {cat.label}
                    </option>
                  ))}
                </select>
                {errors.category && (
                  <p className="mt-1 text-xs text-red-500">{errors.category}</p>
                )}
              </div>

              {/* Description */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Description (Optional)
                </label>
                <textarea
                  value={formData.description}
                  onChange={(e) =>
                    setFormData({ ...formData, description: e.target.value })
                  }
                  placeholder="What does this tag represent?"
                  rows={2}
                  className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
                  disabled={loading}
                />
              </div>

              {/* Color */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Color
                </label>
                <div className="grid grid-cols-5 gap-2">
                  {PRESET_COLORS.map((preset) => (
                    <button
                      key={preset.hex}
                      type="button"
                      onClick={() =>
                        setFormData({ ...formData, color: preset.hex })
                      }
                      className={`h-10 rounded-lg transition-all ${
                        formData.color === preset.hex
                          ? "ring-2 ring-offset-2 ring-blue-500 scale-105"
                          : "hover:scale-105"
                      }`}
                      style={{ backgroundColor: preset.hex }}
                      title={preset.name}
                      disabled={loading}
                    />
                  ))}
                </div>
              </div>

              {/* Footer */}
              <div className="flex items-center justify-end gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
                <button
                  type="button"
                  onClick={handleClose}
                  disabled={loading}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors disabled:opacity-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={loading}
                  className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
                >
                  {loading && <Loader2 className="h-4 w-4 animate-spin" />}
                  Create Tag
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
